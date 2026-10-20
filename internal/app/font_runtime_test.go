package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"zakki/internal/fonts"
	"zakki/locales"
)

// 字体运行时：选项合并、本机键回退与下载任务决策均为纯主线程逻辑，
// 不触网；下载与校验的 I/O 语义在 internal/fonts 的测试里覆盖。

func families(names ...string) []ui.FontFamily {
	out := make([]ui.FontFamily, 0, len(names))
	for _, n := range names {
		out = append(out, ui.FontFamily{Name: n, Monospaced: n == "JetBrains Mono"})
	}
	return out
}

func TestFontOptionsMergeLocalFonts(t *testing.T) {
	a := newTestApp(t)
	a.fonts.families = families("Dank Mono", "SF Mono")
	// 西文：预设 + 本机字体（去重、等宽过滤只在代码字体生效）。
	latin := a.fontOptions(1, "")
	var dank, fira *fontOption
	for i := range latin {
		switch latin[i].key {
		case "local:Dank Mono":
			dank = &latin[i]
		case "fira":
			fira = &latin[i]
		}
	}
	if dank == nil || dank.status != locales.FontInstalled() {
		t.Fatalf("本机字体应免下载进入选项: %+v", latin)
	}
	if fira == nil || fira.status == "" {
		t.Fatalf("未安装的可下载字体应标注状态: %+v", fira)
	}
	// Dank Mono 只来自本机：下载目录里没有它的远程资源。
	if _, ok := fonts.Find("dank"); ok {
		t.Fatal("Dank Mono 不得进入远程下载目录")
	}
	// 代码字体：非等宽的本机字体不出现。
	mono := a.fontOptions(2, "")
	for _, o := range mono {
		if o.key == "local:Dank Mono" {
			t.Fatalf("非等宽字体不应进入代码字体选项: %+v", mono)
		}
	}
	// 已选键不在列表时追加，避免丢选择。
	withCurrent := a.fontOptions(1, "local:Custom Sans")
	found := false
	for _, o := range withCurrent {
		found = found || o.key == "local:Custom Sans"
	}
	if !found {
		t.Fatalf("当前选择应保留在选项中: %+v", withCurrent)
	}
}

func TestUsableFontKeyFallsBackWhenUninstalled(t *testing.T) {
	a := newTestApp(t)
	a.fonts.families = nil
	if got := a.usableFontKey("local:Dank Mono"); got != "system" {
		t.Fatalf("已卸载的本机字体应回退系统: %q", got)
	}
	if a.selectedFontError([]string{"local:Dank Mono"}) == "" {
		t.Fatal("缺失本机字体应给出提示")
	}
	a.fonts.families = families("Dank Mono")
	if got := a.usableFontKey("local:Dank Mono"); got != "local:Dank Mono" {
		t.Fatalf("已安装的本机字体应原样保留: %q", got)
	}
	if got := a.usableFontKey("jetbrains"); got != "jetbrains" {
		t.Fatalf("预设键不做本机回退: %q", got)
	}
}

func TestFontJobsDedupeAndSkipInstalled(t *testing.T) {
	a := newTestApp(t)
	jobs := a.fontJobs([]string{"hand", "kai", "hannotate", "jetbrains", "jbm", "jbm"})
	if len(jobs) != 2 {
		t.Fatalf("同族字体应去重为两个资源: %+v", jobs)
	}
	if jobs[0].resource.ID != "wenkai" || jobs[1].resource.ID != "jetbrains" {
		t.Fatalf("资源映射错误: %+v", jobs)
	}
	a.fonts.families = families("LXGW WenKai Lite", "JetBrains Mono")
	if jobs := a.fontJobs([]string{"hand", "jbm"}); len(jobs) != 0 {
		t.Fatalf("已装字体不应再下载: %+v", jobs)
	}
}
