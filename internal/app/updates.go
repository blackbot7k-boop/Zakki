package app

import (
	"log/slog"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"

	"zakki/locales"
)

func updateOptions() updater.Options {
	return updater.Options{
		Language:               "zh-Hans",
		DisableAutomaticChecks: true,
	}
}

func updatePlugin() mygo.Plugin {
	return native.New(updateOptions())
}

func updateMenuItem() *mygo.MenuItem {
	// 不依赖插件初始化来组装菜单，视图测试无需启动更新器或访问真实用户目录。
	return &mygo.MenuItem{
		ID:    "check-updates",
		Label: locales.MenuCheckUpdates(),
		Click: func(*mygo.MenuItem, *mygo.Window) { updater.CheckForUpdates() },
	}
}

func (a *app) finishQuit(event *mygo.QuitEvent) {
	if err := a.shutdownForQuit(); err != nil {
		// 更新器的重启也经过退出钩子；保存失败时保留运行中的编辑会话。
		event.PreventDefault()
		a.notice = locales.SaveFailedQuitCancelled(err.Error())
		slog.Error("保存失败，已取消退出或更新重启", "error", err)
		if a.ready {
			a.showWindow()
		}
	}
}
