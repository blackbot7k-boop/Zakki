package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 手写字体栈里允许出现的非注册家族名：都是 Apple 之外的普通字体名，
// 未安装时 CoreText 匹配快速返回空，不会像按需下载字体那样在无头环境阻塞。
var handFontSafeFallbacks = map[string]bool{
	"lxgw wenkai": true, // 用户自行安装的完整版
	"霞鹜文楷":        true,
}

// TestHandFontCoversStacks 保证手账主题字体栈引用的每个家族要么已内置注册，
// 要么是已知安全的回退名。栈里出现其他未注册家族名时，无头环境（CI）的文本
// 布局会阻塞在 CoreText 的系统按需字体下载上，测试只会在超时后才失败。
func TestHandFontCoversStacks(t *testing.T) {
	generic := map[string]bool{
		"system-ui": true, "sans-serif": true, "monospace": true, "serif": true,
	}
	for _, stack := range []string{handBodyFont, handHeadingFont, hannotateFont} {
		for _, family := range strings.Split(stack, ",") {
			family = strings.TrimSpace(family)
			if family == "" || generic[family] {
				continue
			}
			key := strings.ToLower(family)
			if key != "lxgw wenkai lite" && !handFontSafeFallbacks[key] {
				t.Fatalf("字体栈 %q 的家族 %q 未内置注册也非安全回退，无头环境会阻塞在系统字体下载", stack, family)
			}
		}
	}
}

// TestRegisterHandFont 内置字体可从资源路径加载并被 MyGo 注册（数据完整、Core Text 可读）。
func TestRegisterHandFont(t *testing.T) {
	data, err := handFontData()
	if err != nil {
		t.Fatalf("读取内置字体失败: %v", err)
	}
	if len(data) < 1<<20 {
		t.Fatalf("内置字体数据异常: %d bytes", len(data))
	}
	if err := ui.RegisterFont(data, "LXGW WenKai Lite"); err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	// registerHandFont 经 sync.Once 幂等；此处再走一遍确认不 panic。
	registerHandFont()
}
