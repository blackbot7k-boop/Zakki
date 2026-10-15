package app

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// 内置手写字体「霞鹜文楷 Lite」（基于 Fontworks Klee One，SIL OFL 1.1，许可见
// resources/OFL-LXGW-WenKai-Lite.txt）。字体文件以单份资源随包分发
// （macOS 在 Contents/Resources，Windows 在 exe 旁），不 go:embed：
// 嵌进 universal 二进制会每个架构各带一份，体积翻倍。
//
// 注册必须只做一次（registerHandFontOnce）：每次 ui.RegisterFont 都会把整份
// 字体数据拷进 CoreText，多注册一个家族名就多一份常驻内存。
// 手账字体栈（view_theme.go）引用注册名 "LXGW WenKai Lite"；「霞鹜文楷」「Klee」
// 等 Apple 之外的名字只作注册失败时的回退——它们的字体匹配会快速返回，
// 不会像按需下载字体（Hannotate SC 等）那样在无头环境无限阻塞。
const handFontFile = "LXGWWenKaiLite-Regular.ttf"

var registerHandFontOnce sync.Once

// registerHandFont 幂等注册内置手写字体；失败只告警，主题回退系统字体。
// 由手写字体栈的解析路径（cjkFontStack/headingFontStack）懒触发：
// 从不用手写主题的用户不付出这份内存。
func registerHandFont() {
	registerHandFontOnce.Do(func() {
		data, err := handFontData()
		if err != nil {
			slog.Warn("读取内置手写字体失败，手写主题回退系统字体", "error", err)
			return
		}
		// 显式家族名与字体自带名都会进注册表；栈里引用显式名保证可查。
		if err := ui.RegisterFont(data, "LXGW WenKai Lite"); err != nil {
			slog.Warn("注册内置手写字体失败，手写主题回退系统字体", "error", err)
		}
	})
}

// handFontData 读取字体文件：成品包在资源目录；go test 与仓库内直跑回退源码树。
func handFontData() ([]byte, error) {
	if dir, err := mygo.App.Path(mygo.PathResources); err == nil && dir != "" {
		if data, err := os.ReadFile(filepath.Join(dir, handFontFile)); err == nil {
			return data, nil
		}
	}
	for _, dir := range []string{"resources", filepath.Join("..", "..", "resources")} {
		if data, err := os.ReadFile(filepath.Join(dir, handFontFile)); err == nil {
			return data, nil
		}
	}
	return nil, os.ErrNotExist
}
