// Package sysnotify 投递一次性系统通知。调度逻辑（何时发）在 app 层的
// reminders；本包只负责"怎么发"，且不依赖 MyGo，可在主线程之外调用。
//
// v1 的平台实现都是"应用内定时器到点 + 系统通知投递"（见 docs/adr/0001）：
// 应用退出后不投递。macOS 走 osascript display notification，Windows 走
// PowerShell WinRT toast；点击通知不产生回调（跳转能力留给后续的
// UNUserNotificationCenter / ScheduledToastNotification 实现）。
package sysnotify

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Notification 是一条系统通知。ID 目前仅供实现记日志/去重，不承载点击回调。
type Notification struct {
	ID    string
	Title string
	Body  string
}

// Notifier 投递一条通知。实现必须可在任意 goroutine 调用（内部可能同步
// 执行外部命令），失败只返回错误，不 panic。
type Notifier interface {
	Notify(n Notification) error
}

// New 返回当前平台的投递器；不支持的平台返回 no-op。
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

// noop is a no-op notifier.
type noopNotifier struct{}

func (noopNotifier) Notify(Notification) error { return nil }

// sanitizeAppleScript 清掉 AppleScript 字符串字面量无法承载的控制字符
// （引号与反斜杠交给 %q 的转义；换行等控制符直接剔除）。
func sanitizeAppleScript(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 {
			return -1
		}
		return r
	}, s)
}

// darwinNotifier 用 osascript display notification 投递；通知归属于
// 系统脚本环境，点击无动作。命令同步执行，调用方应在后台 goroutine 调用。
type darwinNotifier struct{}

func (darwinNotifier) Notify(n Notification) error {
	script := fmt.Sprintf("display notification %q with title %q",
		sanitizeAppleScript(n.Body), sanitizeAppleScript(n.Title))
	return exec.Command("osascript", "-e", script).Run()
}

// toastScript 是 WinRT 原生 toast 的 PowerShell 片段；AUMID 未注册的系统上
// 可能静默不显示，此时依赖应用内 toast 兜底（能力缺口已记录在 ADR）。
const toastScript = `
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
$xml = [Windows.Data.Xml.Dom.XmlDocument]::new()
$xml.LoadXml('%s')
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('%s').Show($toast)
`

// escapeXML 转义 toast XML 的 text 节点内容。
func escapeXML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "'", "&apos;", `"`, "&quot;")
	return r.Replace(s)
}

// windowsNotifier 用 PowerShell WinRT toast 投递；命令同步执行，调用方应在
// 后台 goroutine 调用。
type windowsNotifier struct{}

func (windowsNotifier) Notify(n Notification) error {
	xml := fmt.Sprintf(`<toast activationType="foreground"><visual><binding template="ToastGeneric"><text>%s</text><text>%s</text></binding></visual></toast>`,
		escapeXML(n.Title), escapeXML(n.Body))
	script := fmt.Sprintf(toastScript, xml, escapeXML(n.ID))
	return exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Run()
}
