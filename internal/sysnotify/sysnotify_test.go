package sysnotify

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestAppleScriptUsesArguments(t *testing.T) {
	if !strings.Contains(appleScript, "on run argv") || !strings.Contains(appleScript, "item 2 of argv") || strings.Contains(appleScript, "%q") {
		t.Fatalf("AppleScript 未从参数读取正文: %q", appleScript)
	}
	for _, input := range []string{`" \\ '`, "👨‍💻", "弯引号‘’", "\n\t"} {
		if strings.Contains(appleScript, input) {
			t.Fatalf("用户输入不应出现在脚本: %q", input)
		}
	}
}

func TestToastScriptUsesTextNodesAndFixedAUMID(t *testing.T) {
	for _, part := range []string{"CreateTextNode", "$env:ZAKKI_NOTIFY_TITLE", "$env:ZAKKI_NOTIFY_BODY", powershellAUMID} {
		if !strings.Contains(toastScript, part) {
			t.Fatalf("脚本缺少 %q", part)
		}
	}
	if strings.Contains(toastScript, "zakki-reminder-") || !strings.Contains(powershellPath, `WindowsPowerShell\v1.0\powershell.exe`) {
		t.Fatal("通知 ID 不应作为 AUMID，PowerShell 路径必须固定")
	}
}

func TestEncodedPowerShellUTF16LE(t *testing.T) {
	input := "👨‍💻 弯引号‘’ <&'\""
	encoded := encodedPowerShell(input)
	bytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	units := make([]uint16, len(bytes)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(bytes[i*2:])
	}
	if got := string(utf16.Decode(units)); got != input {
		t.Fatalf("解码 = %q, want %q", got, input)
	}
}

func TestEscapeXML(t *testing.T) {
	if got := escapeXML(`<&'"`); got != `&lt;&amp;&apos;&quot;` {
		t.Fatalf("转义 = %q", got)
	}
}
