package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestHandFontCoversStacks 保证手账主题字体栈引用的每个家族都已内置注册：
// 栈里出现未注册的家族名时，无头环境（CI）的文本布局会阻塞在 CoreText
// 的系统按需字体下载上，测试只会在 10 分钟超时后才失败。
func TestHandFontCoversStacks(t *testing.T) {
	generic := map[string]bool{
		"system-ui": true, "sans-serif": true, "monospace": true, "serif": true,
	}
	registered := make(map[string]bool, len(handFontFamilies))
	for _, family := range handFontFamilies {
		registered[strings.ToLower(family)] = true
	}
	for _, stack := range []string{handBodyFont, handHeadingFont} {
		for _, family := range strings.Split(stack, ",") {
			family = strings.TrimSpace(family)
			if family == "" || generic[family] {
				continue
			}
			if !registered[strings.ToLower(family)] {
				t.Fatalf("字体栈 %q 的家族 %q 未内置注册，无头环境会阻塞在系统字体下载", stack, family)
			}
		}
	}
}

// TestRegisterHandFont 内置字体数据可被 MyGo 注册（数据完整且 Core Text 可读）。
func TestRegisterHandFont(t *testing.T) {
	if len(handFontTTF) < 1<<20 {
		t.Fatalf("内置字体数据异常: %d bytes", len(handFontTTF))
	}
	for _, family := range handFontFamilies {
		if err := ui.RegisterFont(handFontTTF, family); err != nil {
			t.Fatalf("注册 %q 失败: %v", family, err)
		}
	}
	// registerHandFont 经 sync.Once 幂等；此处再走一遍确认不 panic。
	registerHandFont()
}
