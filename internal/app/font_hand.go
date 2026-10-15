package app

import (
	_ "embed"
	"log/slog"
	"sync"

	"github.com/egoist/mygo/ui"
)

// 内置手写字体「霞鹜文楷 Lite」（基于 Fontworks Klee One，SIL OFL 1.1，许可见
// assets/OFL-LXGW-WenKai-Lite.txt），只随应用分发这一份，注册到手账主题字体栈的
// 全部家族名：文楷名（正文）与手札体名（标题的手写感）。手札体本身是 Apple
// 专有字体不可分发，这里由内置文楷接管；不捆绑 Klee、Kaiti SC 等其余字体，
// 栈里也不能再引用未注册的家族名，否则无头环境（CI）的字体匹配会阻塞在
// 系统按需字体下载上。
//
//go:embed assets/LXGWWenKaiLite-Regular.ttf
var handFontTTF []byte

//go:embed assets/OFL-LXGW-WenKai-Lite.txt
var handFontLicense string

// handFontFamilies 必须覆盖手账主题字体栈（view_theme.go）中出现的全部非通用家族名。
var handFontFamilies = []string{
	"LXGW WenKai",
	"霞鹜文楷",
	"Hannotate SC",
}

var registerHandFontOnce sync.Once

// registerHandFont 幂等注册内置手写字体；失败只告警，主题随后回退系统字体。
func registerHandFont() {
	registerHandFontOnce.Do(func() {
		if len(handFontTTF) < 1<<20 {
			slog.Warn("内置手写字体数据异常，跳过注册", "bytes", len(handFontTTF))
			return
		}
		for _, family := range handFontFamilies {
			if err := ui.RegisterFont(handFontTTF, family); err != nil {
				slog.Warn("注册内置手写字体失败", "family", family, "error", err)
			}
		}
	})
}
