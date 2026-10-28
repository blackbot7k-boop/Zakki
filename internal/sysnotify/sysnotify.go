// Package sysnotify 投递一次性系统通知；何时发由 app 层调度。
package sysnotify

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"
)

type Notification struct {
	ID    string
	Title string
	Body  string
}

type Notifier interface{ Notify(Notification) error }

func New() Notifier {
	switch runtime.GOOS {
	case "darwin":
		return darwinNotifier{}
	case "windows":
		return windowsNotifier{}
	default:
		return noopNotifier{}
	}
}

type noopNotifier struct{}

func (noopNotifier) Notify(Notification) error { return nil }

const appleScript = `on run argv
    display notification (item 2 of argv) with title (item 1 of argv)
end run`

type darwinNotifier struct{}

func (darwinNotifier) Notify(n Notification) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "osascript", "-e", appleScript, n.Title, n.Body).Run()
}

const powershellAUMID = `{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe`
const powershellPath = `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`

// 标题和正文仅通过环境变量传入；CreateTextNode 避免脚本字面量和 XML 注入。
const toastScript = `
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
$xml = [Windows.Data.Xml.Dom.XmlDocument]::new()
$xml.LoadXml('<toast><visual><binding template="ToastGeneric"><text></text><text></text></binding></visual></toast>')
$text = $xml.GetElementsByTagName('text')
$text.Item(0).AppendChild($xml.CreateTextNode($env:ZAKKI_NOTIFY_TITLE)) | Out-Null
$text.Item(1).AppendChild($xml.CreateTextNode($env:ZAKKI_NOTIFY_BODY)) | Out-Null
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('` + powershellAUMID + `').Show($toast)
`

func encodedPowerShell(script string) string {
	units := utf16.Encode([]rune(script))
	bytes := make([]byte, len(units)*2)
	for i, unit := range units {
		binary.LittleEndian.PutUint16(bytes[i*2:], unit)
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

type windowsNotifier struct{}

func (windowsNotifier) Notify(n Notification) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, powershellPath, "-NoProfile", "-NonInteractive", "-EncodedCommand", encodedPowerShell(toastScript))
	cmd.Env = append(os.Environ(), "ZAKKI_NOTIFY_TITLE="+n.Title, "ZAKKI_NOTIFY_BODY="+n.Body)
	hideNotificationWindow(cmd)
	return cmd.Run()
}

// 保留兼容测试入口；通知脚本已不依赖任何字符串转义。
func escapeXML(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "'", "&apos;", `"`, "&quot;").Replace(s)
}
