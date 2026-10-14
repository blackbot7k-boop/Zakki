package main

// 持久化层回归测试（state.go 的 load/migrate/save/backup/shutdown 写保护）。
// 全部经 newTestApp 注入 t.TempDir() 与固定时钟，绝不碰真实数据目录。
//
// 与需求的对应：
//   TestPersistenceLoadMissingFile      → 1  文件不存在
//   TestPersistenceLoadFields           → 2  合法 JSON / 缺字段 / 半缺字段
//   TestPersistenceMigrateSavedFile     → 3  version 0 迁移
//   TestPersistenceVersionTooNew        → 4  version > 1 拒绝读取
//   TestPersistenceCorruptFile          → 5  损坏 JSON 隔离与写保护
//   TestPersistenceAtomicSave           → 6  原子保存与目录自动创建
//   TestPersistenceBackupThrottle       → 7  backupNow 绕过节流 / 5 分钟节流
//   TestPersistenceBackupPrune          → 8  备份裁剪到 20 份
//   TestPersistenceShutdown             → 9  退出最终保存与 stopped 闸门
//   TestPersistenceScheduleSaveRace     → 10 防抖落盘期间持续改状态（-race 回归）
//   TestPersistenceClearDot             → 11 先备份后清空，备份失败取消
//   TestPersistenceBackupNameCollision  → 12 同秒多次备份不互覆盖

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
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

// persistDots 构造定长 7 的点内容（不足补空串），用于拼 v1 原始 JSON。
func persistDots(items ...string) []string {
	d := make([]string, DotCount)
	copy(d, items)
	return d
}

// persistTabs 构造与 defaultTabs 的 ID/颜色一致的标签列表（文本可变），
// 用于和落盘结果做 DeepEqual。
func persistTabs(texts ...string) []savedTab {
	defs := defaultTabs()
	for i := range defs {
		if i < len(texts) {
			defs[i].Text = texts[i]
		} else {
			defs[i].Text = ""
		}
	}
	out := make([]savedTab, len(defs))
	for i, tb := range defs {
		out[i] = savedTab{ID: tb.ID, Name: tb.Name, Text: tb.Text, Color: tb.Color}
	}
	return out
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

// persistClock 可推进的假时钟（后台协程可能读，加锁防误报）。
type persistClock struct {
	mu  sync.Mutex
	cur time.Time
}

func (c *persistClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cur
}

func (c *persistClock) advance(d time.Duration) {
	c.mu.Lock()
	c.cur = c.cur.Add(d)
	c.mu.Unlock()
}

func newPersistClock() *persistClock {
	return &persistClock{cur: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
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
			name:       "v2 超过 12 个截断且 active 收敛",
			raw:        `{"version":2,"active":99,"tabs":[` + v2TabJSON(14) + `]}`,
			wantTexts:  nil, // 下面单独断言数量
			wantActive: 11,
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
	cases := []struct {
		name    string
		in      savedFile
		want    func(f savedFile) bool // 断言迁移结果
		wantErr string                 // 非空：期望错误消息含此子串
	}{
		{
			name: "全缺字段补齐 7 个默认标签",
			in:   savedFile{},
			want: func(f savedFile) bool {
				return f.Version == currentDataVersion &&
					reflect.DeepEqual(f.Tabs, persistTabs()) &&
					f.Dots == nil &&
					f.Settings == settings{ThemeSource: "system"}
			},
		},
		{
			name: "v1 dots 按原顺序生成 7 个标签",
			in:   savedFile{Active: 6, Dots: persistDots("x")},
			want: func(f savedFile) bool {
				return f.Version == currentDataVersion && f.Active == 6 &&
					reflect.DeepEqual(f.Tabs, persistTabs("x")) &&
					f.Settings == settings{ThemeSource: "system"}
			},
		},
		{
			name: "v1 半缺 dots 补空",
			in:   savedFile{Active: 2, Dots: []string{"a", "b"}},
			want: func(f savedFile) bool {
				return reflect.DeepEqual(f.Tabs, persistTabs("a", "b"))
			},
		},
		{
			name: "v1 多余 dots 截断到 7",
			in:   savedFile{Dots: []string{"0", "1", "2", "3", "4", "5", "6", "7"}},
			want: func(f savedFile) bool {
				return reflect.DeepEqual(f.Tabs, persistTabs("0", "1", "2", "3", "4", "5", "6"))
			},
		},
		{
			name: "v1 旧版 preview 并入 source",
			in:   savedFile{Settings: settings{Preview: true}},
			want: func(f savedFile) bool {
				return f.Settings == (settings{ThemeSource: "system", Source: true, Preview: true})
			},
		},
		{
			name: "v2 补缺失 ID 与无效颜色",
			in: savedFile{Version: 2, Active: 1, Tabs: []savedTab{
				{Text: "甲", Name: "工作"},
				{ID: 5, Text: "乙", Color: "red!"},
			}},
			want: func(f savedFile) bool {
				return f.Version == currentDataVersion && len(f.Tabs) == 2 &&
					f.Tabs[0].ID > 0 && f.Tabs[0].Name == "工作" &&
					f.Tabs[1].ID > 0 && f.Tabs[0].ID != f.Tabs[1].ID &&
					f.Tabs[1].Color == "" && // 无效颜色清空，渲染回退调色板
					f.Active == 1
			},
		},
		{
			name: "v2 重复 ID 重新分配",
			in: savedFile{Version: 2, Tabs: []savedTab{
				{ID: 3, Text: "a"}, {ID: 3, Text: "b"},
			}},
			want: func(f savedFile) bool {
				return f.Tabs[0].ID != f.Tabs[1].ID && f.Tabs[0].Text == "a" && f.Tabs[1].Text == "b"
			},
		},
		{
			name: "v2 空标签列表补 7 个默认",
			in:   savedFile{Version: 2},
			want: func(f savedFile) bool {
				return reflect.DeepEqual(f.Tabs, persistTabs())
			},
		},
		{
			name: "v2 超过 12 个截断",
			in: savedFile{Version: 2, Tabs: func() []savedTab {
				tabs := make([]savedTab, 15)
				for i := range tabs {
					tabs[i] = savedTab{ID: uint64(i + 1), Text: fmt.Sprintf("t%d", i)}
				}
				return tabs
			}(), Active: 14},
			want: func(f savedFile) bool {
				return len(f.Tabs) == MaxTabCount && f.Active == MaxTabCount-1 &&
					f.Tabs[0].Text == "t0" && f.Tabs[11].Text == "t11"
			},
		},
		{
			name:    "version 高于当前被拒绝",
			in:      savedFile{Version: currentDataVersion + 1},
			wantErr: "当前版本无法读取",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := migrateSavedFile(tc.in)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("错误 = %v, want 含 %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("migrateSavedFile 报错: %v", err)
			}
			if !tc.want(got) {
				t.Fatalf("迁移结果 = %+v", got)
			}
		})
	}

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

// ---- 4. version 高于当前 ----

func TestPersistenceVersionTooNew(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	const raw = `{"version":3,"active":0,"tabs":[{"text":"future"}]}`
	persistWrite(t, a.notes.DataFile, raw)

	err := a.load()
	if err == nil {
		t.Fatal("version 3 应当 load 报错")
	}
	if !strings.Contains(err.Error(), "当前版本无法读取") {
		t.Fatalf("错误消息 = %q, want 含“当前版本无法读取”", err)
	}
	if got := string(persistReadRaw(t, a.notes.DataFile)); got != raw {
		t.Fatalf("原文件被改动: %q, want %q", got, raw)
	}
}

// ---- 5. 损坏 JSON：隔离、写保护、确认写回 ----

func TestPersistenceCorruptFile(t *testing.T) {
	const corruptRaw = `{"version":1,"dots":[这根本不是 JSON`
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	persistWrite(t, a.notes.DataFile, corruptRaw)

	t.Run("load 报错且原文件字节不变", func(t *testing.T) {
		if err := a.load(); err == nil {
			t.Fatal("损坏 JSON 应当 load 报错")
		}
		if got := string(persistReadRaw(t, a.notes.DataFile)); got != corruptRaw {
			t.Fatalf("原文件被改动: %q, want %q", got, corruptRaw)
		}
	})

	t.Run("生成隔离副本且内容为原件", func(t *testing.T) {
		matches, err := filepath.Glob(a.notes.DataFile + ".corrupt-*")
		if err != nil {
			t.Fatalf("Glob 失败: %v", err)
		}
		if len(matches) != 1 {
			t.Fatalf("隔离副本 = %v, want 恰好 1 个", matches)
		}
		if got := string(persistReadRaw(t, matches[0])); got != corruptRaw {
			t.Fatalf("隔离副本 = %q, want %q", got, corruptRaw)
		}
	})

	t.Run("写保护期间 saveNow 只写 .pending", func(t *testing.T) {
		if got := a.currentDataPath(); got != a.pendingPath() {
			t.Fatalf("写保护期间生效路径 = %q, want %q", got, a.pendingPath())
		}
		a.tabs[0].Text = "暂存的新内容"
		if err := a.saveNow(); err != nil {
			t.Fatalf("saveNow 报错: %v", err)
		}
		if got := string(persistReadRaw(t, a.notes.DataFile)); got != corruptRaw {
			t.Fatalf("写保护期间原文件被覆盖: %q, want %q", got, corruptRaw)
		}
		if _, err := os.Stat(a.pendingPath()); err != nil {
			t.Fatalf(".pending 未生成: %v", err)
		}
		if f := persistReadSaved(t, a.pendingPath()); f.Tabs[0].Text != "暂存的新内容" {
			t.Fatalf(".pending 内容 = %q, want 暂存的新内容", f.Tabs[0].Text)
		}
	})

	t.Run("confirmWriteBack 后才写主文件", func(t *testing.T) {
		if err := a.confirmWriteBack(); err != nil {
			t.Fatalf("confirmWriteBack 报错: %v", err)
		}
		if got := a.currentDataPath(); got != a.notes.DataFile {
			t.Fatalf("确认后生效路径 = %q, want %q", got, a.notes.DataFile)
		}
		if f := persistReadSaved(t, a.notes.DataFile); f.Tabs[0].Text != "暂存的新内容" {
			t.Fatalf("主文件内容 = %q, want 暂存的新内容", f.Tabs[0].Text)
		}
		if _, err := os.Stat(a.pendingPath()); !os.IsNotExist(err) {
			t.Fatalf("确认后 .pending 应被清理, stat err = %v", err)
		}
		a.tabs[1].Text = "确认后再改"
		if err := a.saveNow(); err != nil {
			t.Fatalf("saveNow 报错: %v", err)
		}
		if f := persistReadSaved(t, a.notes.DataFile); f.Tabs[1].Text != "确认后再改" {
			t.Fatalf("saveNow 未写主文件: tabs[1] = %q", f.Tabs[1].Text)
		}
	})
}

// ---- 6. 原子保存 ----

func TestPersistenceAtomicSave(t *testing.T) {
	t.Run("主文件是合法 JSON 且含最新内容", func(t *testing.T) {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		want := savedFile{
			Version: currentDataVersion,
			Active:  5,
			Tabs:    persistTabs("中文 ✓ # 标题", "", "第三点\n多行", "", "", "", "尾点"),
			Settings: settings{
				ThemeSource:  "dark",
				AlwaysOnTop:  true,
				HideOnEscape: true,
				Source:       true,
			},
		}
		for i := range a.tabs {
			a.tabs[i].Text = want.Tabs[i].Text
		}
		a.active = want.Active
		a.cfg = want.Settings

		if err := a.saveNow(); err != nil {
			t.Fatalf("saveNow 报错: %v", err)
		}
		if got := persistReadSaved(t, a.notes.DataFile); !reflect.DeepEqual(got, want) {
			t.Fatalf("落盘内容 = %+v, want %+v", got, want)
		}
		if _, err := os.Stat(a.notes.DataFile + ".tmp"); !os.IsNotExist(err) {
			t.Fatalf("原子替换后不应残留 .tmp, stat err = %v", err)
		}
	})

	t.Run("目标目录不存在时自动创建", func(t *testing.T) {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		a.notes.DataFile = filepath.Join(filepath.Dir(a.notes.DataFile), "nested", "deep", "tot-md.json")
		a.tabs[0].Text = "新建目录也能存"

		if err := a.saveNow(); err != nil {
			t.Fatalf("saveNow 报错: %v", err)
		}
		if f := persistReadSaved(t, a.notes.DataFile); f.Tabs[0].Text != "新建目录也能存" {
			t.Fatalf("落盘内容 = %q, want 新建目录也能存", f.Tabs[0].Text)
		}
	})
}

// ---- 7. 备份节流 ----

func TestPersistenceBackupThrottle(t *testing.T) {
	clk := newPersistClock()
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.setTestNow(clk.now)
	a.tabs[0].Text = "备份的内容"
	if err := a.saveNow(); err != nil {
		t.Fatalf("saveNow 报错: %v", err)
	}

	t.Run("backupNow 五分钟内连写两份都落盘", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			if err := a.backupNow(); err != nil {
				t.Fatalf("backupNow 第 %d 次报错: %v", i+1, err)
			}
		}
		if got := persistBackupNames(t, a.notes.BackupDir); len(got) != 2 {
			t.Fatalf("备份数 = %d (%v), want 2", len(got), got)
		}
	})

	t.Run("五分钟内周期备份不新增", func(t *testing.T) {
		before := persistBackupNames(t, a.notes.BackupDir)
		if err := a.maybeWritePeriodicBackup(); err != nil {
			t.Fatalf("maybeWritePeriodicBackup 报错: %v", err)
		}
		if got := persistBackupNames(t, a.notes.BackupDir); !reflect.DeepEqual(got, before) {
			t.Fatalf("节流期内备份被改动: %v, want %v", got, before)
		}
	})

	t.Run("五分钟后周期备份新增", func(t *testing.T) {
		clk.advance(backupInterval + time.Second)
		before := persistBackupNames(t, a.notes.BackupDir)
		if err := a.maybeWritePeriodicBackup(); err != nil {
			t.Fatalf("maybeWritePeriodicBackup 报错: %v", err)
		}
		got := persistBackupNames(t, a.notes.BackupDir)
		if len(got) != len(before)+1 {
			t.Fatalf("备份数 = %d (%v), want %d", len(got), got, len(before)+1)
		}
	})
}

// ---- 8. 备份裁剪 ----

func TestPersistenceBackupPrune(t *testing.T) {
	clk := newPersistClock()
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.setTestNow(clk.now)
	a.tabs[0].Text = "裁剪测试"
	if err := a.saveNow(); err != nil {
		t.Fatalf("saveNow 报错: %v", err)
	}

	const total = 25
	for i := 0; i < total; i++ {
		clk.advance(time.Second) // 每份一秒，字典序即时间序
		if err := a.backupNow(); err != nil {
			t.Fatalf("backupNow 第 %d 次报错: %v", i+1, err)
		}
		got := persistBackupNames(t, a.notes.BackupDir)
		if want := min(i+1, 20); len(got) != want {
			t.Fatalf("第 %d 次后备份数 = %d (%v), want %d", i+1, len(got), got, want)
		}
	}

	kept := persistBackupNames(t, a.notes.BackupDir)
	if len(kept) != 20 {
		t.Fatalf("最终备份数 = %d (%v), want 20", len(kept), kept)
	}
	// 每次备份前先拨 1 秒：备份时间戳是 12:00:01..12:00:25。
	// 留下的应是最新 20 份（12:00:06..12:00:25），最早的 5 份（12:00:01..12:00:05）被裁掉。
	for _, name := range kept {
		for sec := 1; sec <= 5; sec++ {
			old := fmt.Sprintf("tot-md-%s.json", time.Date(2024, 6, 1, 12, 0, sec, 0, time.UTC).Format("20060102-150405"))
			if name == old {
				t.Fatalf("旧备份 %s 不应保留（%v）", name, kept)
			}
		}
	}
	newest := fmt.Sprintf("tot-md-%s.json", time.Date(2024, 6, 1, 12, 0, 25, 0, time.UTC).Format("20060102-150405"))
	if kept[len(kept)-1] != newest {
		t.Fatalf("最新备份 = %q, want %q（列表 %v）", kept[len(kept)-1], newest, kept)
	}
}

// ---- 9. shutdown ----

func TestPersistenceShutdown(t *testing.T) {
	t.Run("最终保存落盘", func(t *testing.T) {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		want := savedFile{
			Version: currentDataVersion,
			Active:  4,
			Tabs:    persistTabs("退出前的最终内容"),
			Settings: settings{
				ThemeSource:  "dark",
				AlwaysOnTop:  true,
				HideOnEscape: true,
				Source:       true,
			},
		}
		for i := range a.tabs {
			a.tabs[i].Text = want.Tabs[i].Text
		}
		a.active = want.Active
		a.cfg = want.Settings

		a.shutdown()
		if got := persistReadSaved(t, a.notes.DataFile); !reflect.DeepEqual(got, want) {
			t.Fatalf("最终落盘 = %+v, want %+v", got, want)
		}
	})

	t.Run("排队 pending 被最终快照取代", func(t *testing.T) {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		a.tabs[0].Text = "排队的旧快照"
		a.scheduleSave() // 防抖排队（800ms 后才写）
		a.tabs[0].Text = "退出时的新快照"
		a.active = 2

		a.shutdown()
		f := persistReadSaved(t, a.notes.DataFile)
		if f.Tabs[0].Text != "退出时的新快照" || f.Active != 2 {
			t.Fatalf("最终落盘 = %+v, want 退出时的新快照/active=2", f)
		}
		time.Sleep(950 * time.Millisecond) // 过防抖窗口，确认旧 pending 不再落盘
		if f := persistReadSaved(t, a.notes.DataFile); f.Tabs[0].Text != "退出时的新快照" || f.Active != 2 {
			t.Fatalf("防抖到期后被旧快照覆盖: %+v", f)
		}
	})

	t.Run("退出后保存请求静默忽略", func(t *testing.T) {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		a.tabs[0].Text = "退出前内容"
		a.shutdown()
		raw := persistReadRaw(t, a.notes.DataFile)

		a.tabs[0].Text = "退出后内容"
		if err := a.saveNow(); err != nil {
			t.Fatalf("退出后 saveNow 应静默忽略，得到错误: %v", err)
		}
		a.scheduleSave()
		if err := a.backupNow(); err != nil {
			t.Fatalf("退出后 backupNow 应静默忽略，得到错误: %v", err)
		}
		time.Sleep(1200 * time.Millisecond) // 过防抖窗口
		if got := persistReadRaw(t, a.notes.DataFile); string(got) != string(raw) {
			t.Fatalf("退出后数据文件被改动: %q, want %q", got, raw)
		}
		if got := persistBackupNames(t, a.notes.BackupDir); len(got) != 0 {
			t.Fatalf("退出后备份被改动: %v, want 无", got)
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
		names := persistBackupNames(t, a.notes.BackupDir)
		if len(names) != 1 {
			t.Fatalf("备份 = %v, want 1 份", names)
		}
		if raw := string(persistReadRaw(t, filepath.Join(a.notes.BackupDir, names[0]))); !strings.Contains(raw, "清空前应进备份") {
			t.Fatalf("备份不含清空前内容（应先备份后清空）: %q", raw)
		}
	})

	t.Run("备份失败取消清空", func(t *testing.T) {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		blocker := filepath.Join(filepath.Dir(a.notes.DataFile), "blocked")
		persistWrite(t, blocker, "我是普通文件，MkdirAll 必须失败")
		a.notes.BackupDir = blocker
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

// ---- 12. 同秒多次备份文件名不冲突 ----

func TestPersistenceBackupNameCollision(t *testing.T) {
	a := newTestApp(t) // 固定时钟：三次备份落在同一秒
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "同秒备份"
	if err := a.saveNow(); err != nil {
		t.Fatalf("saveNow 报错: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := a.backupNow(); err != nil {
			t.Fatalf("backupNow 第 %d 次报错: %v", i+1, err)
		}
	}

	names := persistBackupNames(t, a.notes.BackupDir)
	if len(names) != 3 {
		t.Fatalf("备份 = %v, want 3 份都在", names)
	}
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			t.Fatalf("备份名重复: %v", names)
		}
		seen[n] = true
	}
	for _, n := range names {
		f := persistReadSaved(t, filepath.Join(a.notes.BackupDir, n))
		if len(f.Tabs) == 0 || f.Tabs[0].Text != "同秒备份" {
			t.Fatalf("备份 %s 内容 = %+v, want 同秒备份", n, f.Tabs)
		}
	}
}
