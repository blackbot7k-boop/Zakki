package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"
)

func TestZakkiIdentityKeepsExistingData(t *testing.T) {
	if appName != "Zakki" || !strings.HasPrefix(welcomeText, "# Zakki\n") {
		t.Fatal("应用名与欢迎文字应使用 Zakki")
	}
	if dataDirectoryName != "Zakki" || legacyDataDirectoryName != "Tot MD" {
		t.Fatal("数据目录应更名并保留旧目录名供迁移")
	}
	if dataFileName != "zakki.json" || legacyDataFileName != "tot-md.json" {
		t.Fatal("数据文件应更名并保留旧文件名供迁移")
	}
	raw, err := os.ReadFile("../../mygo.json")
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Name       string `json:"name"`
		Identifier string `json:"identifier"`
		Updates    struct {
			GitHub    string `json:"github"`
			PublicKey string `json:"publicKey"`
		} `json:"updates"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	if config.Name != appName || config.Identifier != "com.gomeen.zakki" {
		t.Fatalf("安装包应使用 Zakki 名称与标识: %+v", config)
	}
	if config.Updates.GitHub != "gomeen/Zakki" || config.Updates.PublicKey == "" {
		t.Fatal("安装包必须订阅 Zakki 的签名更新")
	}
}

func TestUpdateMenuAndNativePlugin(t *testing.T) {
	a := newTestApp(t)
	menu := a.createMenu()
	item := menu.ItemByID("check-updates")
	if item == nil || item.Label != "检查更新…" || item.Click == nil {
		t.Fatalf("缺少更新入口: %+v", item)
	}
	if p := updatePlugin(); p.Name != "updater" || p.Setup == nil {
		t.Fatal("必须安装 MyGo 原生更新插件")
	}
	if opts := updateOptions(); opts.Language != "zh-Hans" || !opts.DisableAutomaticChecks {
		t.Fatalf("首版只提供中文手动检查，不默认联网检查: %+v", opts)
	}
}

func TestQuitSavesBeforeUpdateRelaunch(t *testing.T) {
	a := newTestApp(t)
	a.tabs[0].Text = "重启前未保存的笔记"
	a.prefs.Window.AlwaysOnTop = true
	event := &mygo.QuitEvent{}
	a.finishQuit(event)
	if event.DefaultPrevented() {
		t.Fatal("保存成功不应阻止更新重启")
	}
	raw, err := os.ReadFile(a.notes.DataFile)
	if err != nil {
		t.Fatal(err)
	}
	file, _, err := a.notes.ParseRaw(raw)
	if err != nil || file.Tabs[0].Text != a.tabs[0].Text {
		t.Fatalf("退出前应保存笔记: %+v %v", file, err)
	}
	raw, err = os.ReadFile(a.preferencesPath())
	if err != nil {
		t.Fatal(err)
	}
	prefs, err := decodePreferences(raw)
	if err != nil || !prefs.Window.AlwaysOnTop {
		t.Fatalf("退出前应保存偏好: %+v %v", prefs, err)
	}
}

func TestQuitFailureCancelsUpdateRelaunchAndAllowsRetry(t *testing.T) {
	for _, failure := range []string{"notes", "preferences"} {
		t.Run(failure, func(t *testing.T) {
			a := newTestApp(t)
			blocker := filepath.Join(t.TempDir(), "not-a-directory")
			if err := os.WriteFile(blocker, []byte("保留"), 0o600); err != nil {
				t.Fatal(err)
			}
			dataPath, prefsPath := a.notes.DataFile, a.prefsStore.Path
			if failure == "notes" {
				a.notes.DataFile = filepath.Join(blocker, "zakki.json")
			} else {
				a.prefsStore.Path = filepath.Join(blocker, "preferences.json")
			}
			event := &mygo.QuitEvent{}
			a.finishQuit(event)
			if !event.DefaultPrevented() || !strings.Contains(a.notice, "保存失败") {
				t.Fatal("任一种保存失败必须取消退出/重启并提示")
			}
			a.notes.DataFile, a.prefsStore.Path = dataPath, prefsPath
			a.tabs[0].Text = "取消退出后继续编辑"
			if err := a.saveNow(); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(dataPath)
			if err != nil {
				t.Fatal(err)
			}
			file, _, err := a.notes.ParseRaw(raw)
			if err != nil || file.Tabs[0].Text != a.tabs[0].Text {
				t.Fatal("取消退出后 Store 必须仍可保存新改动")
			}
			event = &mygo.QuitEvent{}
			a.finishQuit(event)
			if event.DefaultPrevented() {
				t.Fatal("修复写盘问题后应能再次退出")
			}
		})
	}
}

func TestUpdateRelaunchPreservesProtectedData(t *testing.T) {
	a := newTestApp(t)
	original := []byte("不可覆盖的损坏原件")
	if err := os.WriteFile(a.notes.DataFile, original, 0o600); err != nil {
		t.Fatal(err)
	}
	a.notes.SetSaveBlocked(true)
	a.tabs[0].Text = "暂存的笔记"
	event := &mygo.QuitEvent{}
	a.finishQuit(event)
	if event.DefaultPrevented() {
		t.Fatal("写保护期间仍可保存暂存文件后退出")
	}
	raw, err := os.ReadFile(a.notes.DataFile)
	if err != nil || string(raw) != string(original) {
		t.Fatal("更新重启不应覆盖写保护原件")
	}
	raw, err = os.ReadFile(a.pendingPath())
	if err != nil || !strings.Contains(string(raw), "暂存的笔记") {
		t.Fatal("退出前必须保存暂存笔记")
	}
}
