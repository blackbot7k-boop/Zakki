package app

// 应用层持久化回归：载入后的标签/配置映射、欢迎文本、清空前备份。
// 保存、备份、写保护与退出闸门在 internal/notes。
// 全部经 newTestApp 注入 t.TempDir() 与固定时钟，绝不碰真实数据目录。
//
// 仍留在本文件：载入映射、欢迎文本、清空标签、防抖与 app 状态的竞态。
// 版本拒绝、损坏隔离、原子保存、备份节流/裁剪/同秒文件名、退出闸门见 internal/notes。

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

func persistBackupNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("读取备份目录 %s 失败: %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
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
	if _, err := os.Stat(a.notes.DataFile); !os.IsNotExist(err) {
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
			persistWrite(t, a.notes.DataFile, tc.raw)

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
		persistWrite(t, a.notes.DataFile, `{"active":99,"dots":["x"],"settings":{"preview":true}}`)

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
		f := persistReadSaved(t, a.notes.DataFile)
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
		persistWrite(t, a.notes.DataFile, `{"version":2,"active":0,"tabs":[{"text":"a"},{"id":4,"text":"b","color":"#00a3b4"}]}`)
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

// ---- 10. 防抖落盘期间持续改状态（-race 回归） ----

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
			if _, err := os.Stat(a.notes.DataFile); err == nil {
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
		t.Fatal("防抖定时器 5 秒内未落盘")
	}

	f := persistReadSaved(t, a.notes.DataFile) // 不是合法 JSON 会在这里判失败
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
	t.Run("先备份后清空", func(t *testing.T) {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		a.active = 2
		a.tabs[2].Text = "清空前应进备份"
		a.tabs[0].Text = "其他点不动"

		a.clearActiveTab()

		if a.tabs[2].Text != "" {
			t.Fatalf("当前点未被清空: %q", a.tabs[2].Text)
		}
		if a.tabs[0].Text != "其他点不动" {
			t.Fatalf("其他点被误改: %q", a.tabs[0].Text)
		}
		names := persistBackupNames(t, a.notes.BackupDir())
		if len(names) != 1 {
			t.Fatalf("备份 = %v, want 1 份", names)
		}
		if raw := string(persistReadRaw(t, filepath.Join(a.notes.BackupDir(), names[0]))); !strings.Contains(raw, "清空前应进备份") {
			t.Fatalf("备份不含清空前内容（应先备份后清空）: %q", raw)
		}
	})

	t.Run("备份失败取消清空", func(t *testing.T) {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		blocker := filepath.Join(filepath.Dir(a.notes.DataFile), "blocked")
		persistWrite(t, blocker, "我是普通文件，MkdirAll 必须失败")
		a.notes.SetBackupDir(blocker) // 选择的备份文件夹不可写
		a.active = 3
		a.tabs[3].Text = "备份失败要保留"

		a.clearActiveTab()

		if a.tabs[3].Text != "备份失败要保留" {
			t.Fatalf("备份失败仍被清空: %q", a.tabs[3].Text)
		}
		if !strings.Contains(a.saveError(), "备份失败") {
			t.Fatalf("备份失败未上报: saveError = %q", a.saveError())
		}
	})
}
