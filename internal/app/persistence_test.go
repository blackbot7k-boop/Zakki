package app

// 应用层持久化回归：载入映射、欢迎文本、单文件保存与清空标签。
// 全部经 newTestApp 注入临时目录与固定时钟。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// ---- 测试小工具 ----

func persistWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入 %s 失败: %v", path, err)
	}
}

func persistReadRaw(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", path, err)
	}
	return raw
}

// persistReadSaved 读入并解析数据文件；不是合法 JSON 直接判失败。
func persistReadSaved(t *testing.T, path string) savedFile {
	t.Helper()
	raw := persistReadRaw(t, path)
	var f savedFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("解析 %s 失败: %v（内容 %q）", path, err, raw)
	}
	return f
}

// assertOnlyStorageFiles 拒绝副本、暂存、备份目录和遗留探针。
func assertOnlyStorageFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, entry := range entries {
		got = append(got, entry.Name())
	}
	sort.Strings(got)
	sort.Strings(names)
	if !reflect.DeepEqual(got, names) {
		t.Fatalf("目录 %s = %v, want 仅 %v", dir, got, names)
	}
}

// tabTexts 取内存里全部标签文本（断言用）。
func tabTexts(a *app) []string {
	texts := make([]string, len(a.tabs))
	for i := range a.tabs {
		texts[i] = a.tabs[i].Text
	}
	return texts
}

// v2TabJSON 拼出 n 个 v2 标签的 JSON 数组内容（text=t0..t<n>）。
func v2TabJSON(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = fmt.Sprintf(`{"text":"t%d"}`, i)
	}
	return strings.Join(parts, ",")
}

// ---- 1. 文件不存在 ----

func TestPersistenceLoadMissingFile(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)

	if err := a.load(); err != nil {
		t.Fatalf("首次运行 load 应成功，得到错误: %v", err)
	}

	if len(a.tabs) != DefaultTabCount {
		t.Fatalf("默认标签数 = %d, want %d", len(a.tabs), DefaultTabCount)
	}
	if a.tabs[0].Text != welcomeText {
		t.Fatalf("默认标签 1 内容 = %q, want 欢迎文本", a.tabs[0].Text)
	}
	if a.nextTabID != DefaultTabCount+1 {
		t.Fatalf("nextTabID = %d, want %d", a.nextTabID, DefaultTabCount+1)
	}
	if a.active != 0 {
		t.Fatalf("默认 active = %d, want 0", a.active)
	}
	if a.cfg.ThemeSource != "system" {
		t.Fatalf("默认 themeSource = %q, want system", a.cfg.ThemeSource)
	}
	if _, err := os.Stat(a.notes.DataPath()); !os.IsNotExist(err) {
		t.Fatalf("load 不应创建数据文件, stat err = %v", err)
	}
}

// ---- 2. 合法 JSON 与缺字段默认值 ----

func TestPersistenceLoadFields(t *testing.T) {
	const fullDots = `["d0","d1","d2","d3","d4","d5","d6"]`
	seven := []string{"d0", "d1", "d2", "d3", "d4", "d5", "d6"}
	cases := []struct {
		name       string
		raw        string
		wantTexts  []string
		wantActive int
		wantCfg    settings
	}{
		{
			name:       "完整字段全部读入",
			raw:        `{"version":1,"active":3,"dots":` + fullDots + `,"settings":{"themeSource":"dark","alwaysOnTop":true,"hideOnEscape":true,"source":true}}`,
			wantTexts:  seven,
			wantActive: 3,
			wantCfg:    settings{ThemeSource: "dark", AlwaysOnTop: true, HideOnEscape: true, Source: true},
		},
		{
			name:       "缺 settings 用默认值",
			raw:        `{"version":1,"active":1,"dots":` + fullDots + `}`,
			wantTexts:  seven,
			wantActive: 1,
			wantCfg:    settings{ThemeSource: "system", HideOnEscape: true},
		},
		{
			name:       "缺 dots 给欢迎文本",
			raw:        `{"version":1,"active":2}`,
			wantTexts:  []string{welcomeText, "", "", "", "", "", ""},
			wantActive: 2,
			wantCfg:    settings{ThemeSource: "system", HideOnEscape: true},
		},
		{
			name:       "半缺 dots 补空串",
			raw:        `{"version":1,"active":5,"dots":["a","b","c"]}`,
			wantTexts:  []string{"a", "b", "c", "", "", "", ""},
			wantActive: 5,
			wantCfg:    settings{ThemeSource: "system", HideOnEscape: true},
		},
		{
			name:       "多余 dots 截断到 7",
			raw:        `{"version":1,"active":0,"dots":["d0","d1","d2","d3","d4","d5","d6","d7","d8"]}`,
			wantTexts:  seven,
			wantActive: 0,
			wantCfg:    settings{ThemeSource: "system", HideOnEscape: true},
		},
		{
			name:       "active 过大收敛到 6",
			raw:        `{"version":1,"active":99,"dots":` + fullDots + `}`,
			wantTexts:  seven,
			wantActive: 6,
			wantCfg:    settings{ThemeSource: "system", HideOnEscape: true},
		},
		{
			name:       "active 为负收敛到 0",
			raw:        `{"version":1,"active":-9,"dots":` + fullDots + `}`,
			wantTexts:  seven,
			wantActive: 0,
			wantCfg:    settings{ThemeSource: "system", HideOnEscape: true},
		},
		{
			name:       "旧版 preview 并入 source",
			raw:        `{"version":1,"active":0,"dots":` + fullDots + `,"settings":{"preview":true}}`,
			wantTexts:  seven,
			wantActive: 0,
			wantCfg:    settings{ThemeSource: "system", HideOnEscape: true, Source: true},
		},
		{
			name:       "显式 hideOnEscape:false 保留",
			raw:        `{"version":1,"active":0,"dots":` + fullDots + `,"settings":{"hideOnEscape":false}}`,
			wantTexts:  seven,
			wantActive: 0,
			wantCfg:    settings{ThemeSource: "system"},
		},
		{
			name:       "v2 tabs 读入并补 ID/颜色",
			raw:        `{"version":2,"active":1,"tabs":[{"text":"甲","name":"工作"},{"text":"乙"},{"id":9,"text":"丙","color":"#123abc"}],"settings":{"themeSource":"dark"}}`,
			wantTexts:  []string{"甲", "乙", "丙"},
			wantActive: 1,
			wantCfg:    settings{ThemeSource: "dark", HideOnEscape: true},
		},
		{
			name:       "v2 缺 tabs 视为首次运行",
			raw:        `{"version":2,"active":0}`,
			wantTexts:  []string{welcomeText, "", "", "", "", "", ""},
			wantActive: 0,
			wantCfg:    settings{ThemeSource: "system", HideOnEscape: true},
		},
		{
			name:       "v2 超过上限截断且 active 收敛",
			raw:        `{"version":2,"active":99,"tabs":[` + v2TabJSON(14) + `]}`,
			wantTexts:  nil, // 下面单独断言数量
			wantActive: MaxTabCount - 1,
			wantCfg:    settings{ThemeSource: "system", HideOnEscape: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			persistWrite(t, a.notes.DataPath(), tc.raw)

			if err := a.load(); err != nil {
				t.Fatalf("load 报错: %v", err)
			}
			if tc.wantTexts != nil {
				got := tabTexts(a)
				if !reflect.DeepEqual(got, tc.wantTexts) {
					t.Fatalf("tabs = %q, want %q", got, tc.wantTexts)
				}
			} else if len(a.tabs) != MaxTabCount {
				t.Fatalf("标签数 = %d, want 截断到 %d", len(a.tabs), MaxTabCount)
			}
			if a.active != tc.wantActive {
				t.Fatalf("active = %d, want %d", a.active, tc.wantActive)
			}
			if a.cfg != tc.wantCfg {
				t.Fatalf("cfg = %+v, want %+v", a.cfg, tc.wantCfg)
			}
			// v2 也保证 ID 唯一且计数器就位。
			seen := map[uint64]bool{}
			for i := range a.tabs {
				if a.tabs[i].ID == 0 || seen[a.tabs[i].ID] {
					t.Fatalf("标签 ID 非法或重复: %+v", a.tabs)
				}
				seen[a.tabs[i].ID] = true
			}
			if a.nextTabID <= a.tabs[len(a.tabs)-1].ID {
				t.Fatalf("nextTabID = %d, 应大于最大 ID", a.nextTabID)
			}
		})
	}
}

// ---- 3. v1 → v2 迁移与 v2 修复 ----

func TestPersistenceMigrateSavedFile(t *testing.T) {
	// 纯迁移表在 internal/notes. 这里只保留经 load/save 落到 app 的路径。
	t.Run("load 旧文件后保存升到 version 2 且只写 tabs", func(t *testing.T) {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		persistWrite(t, a.notes.DataPath(), `{"active":99,"dots":["x"],"settings":{"preview":true}}`)

		if err := a.load(); err != nil {
			t.Fatalf("load 报错: %v", err)
		}
		if a.tabs[0].Text != "x" || a.tabs[1].Text != "" || len(a.tabs) != DefaultTabCount {
			t.Fatalf("tabs = %q, want [x  …] 共 7 个", tabTexts(a))
		}
		if a.active != 6 {
			t.Fatalf("active = %d, want 6", a.active)
		}
		if !a.cfg.Source || a.cfg.ThemeSource != "system" {
			t.Fatalf("cfg = %+v, want Source=true 且 themeSource=system", a.cfg)
		}
		if err := a.saveNow(); err != nil {
			t.Fatalf("saveNow 报错: %v", err)
		}
		f := persistReadSaved(t, a.notes.DataPath())
		if f.Version != currentDataVersion {
			t.Fatalf("保存后 version = %d, want %d", f.Version, currentDataVersion)
		}
		if f.Dots != nil {
			t.Fatalf("新保存不应再写 dots: %v", f.Dots)
		}
		if len(f.Tabs) != DefaultTabCount || f.Tabs[0].Text != "x" {
			t.Fatalf("保存的 tabs = %+v", f.Tabs)
		}
	})

	t.Run("v2 缺失的 ID/颜色在 load 后补齐且计数器就位", func(t *testing.T) {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		persistWrite(t, a.notes.DataPath(), `{"version":2,"active":0,"tabs":[{"text":"a"},{"id":4,"text":"b","color":"#00a3b4"}]}`)
		if err := a.load(); err != nil {
			t.Fatalf("load 报错: %v", err)
		}
		if len(a.tabs) != 2 || a.tabs[0].ID == 0 || a.tabs[1].ID != 4 {
			t.Fatalf("tabs = %+v", a.tabs)
		}
		if a.tabs[0].ID == a.tabs[1].ID {
			t.Fatal("ID 不应重复")
		}
		if a.nextTabID <= a.tabs[1].ID {
			t.Fatalf("nextTabID = %d, 应大于现有最大 ID", a.nextTabID)
		}
		// 无效颜色在 load 阶段已清空（渲染回退调色板）。
		a.tabs[1].Color = "invalid!"
		a.tabs[1].ID = 0
		f, err := migrateSavedFile(a.snapshotFile())
		if err != nil {
			t.Fatal(err)
		}
		if f.Tabs[1].Color != "" || f.Tabs[1].ID == 0 {
			t.Fatalf("保存时未修复: %+v", f.Tabs[1])
		}
	})
}

// ---- 固定截止保存期间持续改状态（-race 回归） ----

func TestPersistenceScheduleSaveRace(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	for i := range a.tabs {
		a.tabs[i].Text = fmt.Sprintf("seed-%d", i)
	}
	a.active = 3
	a.scheduleSave()

	// 主线程语义：定时器到期前后持续改状态，绝不等待后台。
	deadline := time.Now().Add(5 * time.Second)
	landed := false
	var overlapUntil time.Time
	for i := 0; time.Now().Before(deadline); i++ {
		a.tabs[i%len(a.tabs)].Text = fmt.Sprintf("mut-%d", i)
		a.active = i % len(a.tabs)
		if !landed {
			if _, err := os.Stat(a.notes.DataPath()); err == nil {
				landed = true
				// 落盘后继续改一小段时间，与后台收尾重叠，给 -race 留抓窗口。
				overlapUntil = time.Now().Add(200 * time.Millisecond)
			}
		} else if time.Now().After(overlapUntil) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !landed {
		t.Fatal("保存定时器 5 秒内未落盘")
	}

	f := persistReadSaved(t, a.notes.DataPath()) // 不是合法 JSON 会在这里判失败
	want := []string{"seed-0", "seed-1", "seed-2", "seed-3", "seed-4", "seed-5", "seed-6"}
	got := make([]string, len(f.Tabs))
	for i := range f.Tabs {
		got[i] = f.Tabs[i].Text
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("落盘 = %q, want 安排时的不可变快照 %q", got, want)
	}
	if f.Active != 3 || f.Version != currentDataVersion {
		t.Fatalf("落盘 = %+v, want active=3 version=%d", f, currentDataVersion)
	}
}

// ---- 11. clearDot ----

func TestPersistenceClearDot(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.active = 2
	a.tabs[2].Text = "清空前内容"
	a.tabs[0].Text = "其他点不动"
	a.resyncLive(-1)
	a.clearActiveTab()
	if a.tabs[2].Text != "" {
		t.Fatalf("当前标签未清空: %q", a.tabs[2].Text)
	}
	if a.tabs[0].Text != "其他点不动" {
		t.Fatalf("其他标签被误改: %q", a.tabs[0].Text)
	}
	if err := a.saveNow(); err != nil {
		t.Fatal(err)
	}
	f := persistReadSaved(t, a.notes.DataPath())
	if len(f.Tabs) != DefaultTabCount || f.Tabs[2].Text != "" || f.Tabs[0].Text != "其他点不动" {
		t.Fatalf("清空结果未完整保存: %+v", f.Tabs)
	}
	assertOnlyStorageFiles(t, filepath.Dir(a.notes.DataPath()), "zakki.json")
}

func TestPersistenceSaveCurrentWritesAllTabsWithoutCopies(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	for i := range a.tabs {
		a.tabs[i].Text = fmt.Sprintf("标签-%d", i)
	}
	a.active = 3
	a.saveCurrent()
	f := persistReadSaved(t, a.notes.DataPath())
	if len(f.Tabs) != len(a.tabs) || f.Active != a.active {
		t.Fatalf("快照不完整: %+v", f)
	}
	for i := range f.Tabs {
		if f.Tabs[i].Text != a.tabs[i].Text {
			t.Fatalf("标签 %d = %q", i, f.Tabs[i].Text)
		}
	}
	assertOnlyStorageFiles(t, filepath.Dir(a.notes.DataPath()), "zakki.json")
}

func TestPersistenceScheduleSaveUsesFixedDeadline(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	if got := defaultPreferences().Storage.SaveIntervalMS; got != 2000 {
		t.Fatalf("默认保存间隔 = %d, want 2000", got)
	}
	a.prefs.Storage.SaveIntervalMS = 300
	start := time.Now()
	a.tabs[0].Text = "首次编辑"
	a.scheduleSave()
	deadline := start.Add(3 * time.Second)
	for i := 0; time.Now().Before(deadline); i++ {
		a.tabs[0].Text = fmt.Sprintf("持续编辑-%d", i)
		a.scheduleSave()
		if _, err := os.Stat(a.notes.DataPath()); err == nil {
			if elapsed := time.Since(start); elapsed < 250*time.Millisecond {
				t.Fatalf("300ms 截止前提前保存: %v", elapsed)
			}
			if f := persistReadSaved(t, a.notes.DataPath()); !strings.HasPrefix(f.Tabs[0].Text, "持续编辑-") {
				t.Fatalf("到期应保存最新快照: %+v", f.Tabs)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("连续编辑不应重置最初 2 秒的保存截止时间")
}
