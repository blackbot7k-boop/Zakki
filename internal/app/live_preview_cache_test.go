package app

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"zakki/internal/codehl"
)

func TestPreviewProjectionCache(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	for _, raw := range []string{"中文😀 **粗体** 和 [链接](https://example.com)", "## 标题中文😀", "- [ ] 中文任务"} {
		a.tabs[0].Text = raw
		blk := a.mdBlocks()[0]
		got := a.previewProjection(blk)
		want := projectBlock(blk)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("投影不同: %q", raw)
		}
		entry := a.previewBlock(blk)
		a.previewProjection(blk)
		if a.previewBlock(blk) != entry {
			t.Fatal("稳态未复用块缓存")
		}
		for i := 0; i <= len(got.units); i++ {
			if got.sourceOffset(i) != want.sourceOffset(i) || got.visibleOffset(i) != want.visibleOffset(i) {
				t.Fatalf("rune 映射不同: %q %d", raw, i)
			}
		}
	}
}

func TestPreviewCacheGenerationAndBound(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = strings.Repeat("中文 **正文**\n\n", livePreviewCacheLimit*3)
	blocks := a.mdBlocks()
	for _, blk := range blocks {
		a.previewProjection(blk)
	}
	if len(a.previewCache.blocks) > livePreviewCacheLimit {
		t.Fatal("块缓存越界")
	}
	old := a.previewBlock(blocks[0])
	offsets := a.mdCacheOffsets
	wantOffsets := append([]int(nil), offsets...)
	a.previewProjection(blocks[0])
	a.tabs[0].Text = "新文档😀"
	fresh := a.mdBlocks()[0]
	if !reflect.DeepEqual(offsets, wantOffsets) {
		t.Fatal("重切覆写仍被当前帧持有的上一代偏移表")
	}
	if a.previewProjection(fresh).text != "新文档😀" || a.previewBlock(fresh) == old || len(a.previewCache.blocks) != 1 {
		t.Fatal("编辑后未清除旧代缓存")
	}
	a.invalidateMdCache()
	a.mdBlocks()
	if a.previewCache.blocks != nil {
		t.Fatal("显式失效保留旧代缓存")
	}
	old = a.previewBlock(fresh)
	a.active = 1
	a.tabs[1].Text = "新文档😀"
	fresh = a.mdBlocks()[0]
	if a.previewBlock(fresh) == old {
		t.Fatal("相同文本切标签未清除缓存")
	}
	for i := 0; i < 100; i++ {
		a.tabs[1].Text = fmt.Sprintf("版本 %d", i)
		a.previewProjection(a.mdBlocks()[0])
		if len(a.previewCache.blocks) != 1 {
			t.Fatal("缓存保留历史编辑版本")
		}
	}
}

func TestPreviewSnapshotCachedOffsets(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	for _, doc := range []string{"甲😀\n\n## 中文标题\n\n末尾", "", "```go\n中文😀\n```\n"} {
		a.tabs[0].Text = doc
		blocks := a.mdBlocks()
		for active := -1; active <= len(blocks)+1; active++ {
			a.mdActive = active
			a.mdCaret, a.mdSelEnd = 1, 3
			a.selSyncStart, a.selSyncEnd = -1, -1
			off := 0
			for i, b := range blocks {
				if i == active {
					break
				}
				off += utf8.RuneCountInString(b.raw) + 1
			}
			snap := a.liveSnapshot()
			if snap.start != off+1 || snap.end != off+3 {
				t.Fatalf("active=%d 偏移=%d..%d want=%d..%d", active, snap.start, snap.end, off+1, off+3)
			}
			a.selSyncStart, a.selSyncEnd = 0, 2
			snap = a.liveSnapshot()
			if snap.start != off || snap.end != off+2 {
				t.Fatal("同步选区偏移错误")
			}
		}
	}
	a.selDoc = true
	a.selDocStart, a.selDocEnd = 1, 4
	if s := a.liveSnapshot(); s.start != 1 || s.end != 4 {
		t.Fatal("文档选区被块偏移污染")
	}
	a.selDoc = false
	a.cfg.Source = true
	a.srcCaret, a.srcSelEnd = 2, 5
	a.srcSelSyncStart = -1
	if s := a.liveSnapshot(); s.start != 2 || s.end != 5 {
		t.Fatal("源码选区被块偏移污染")
	}
}

func TestPreviewCodePaletteInvalidation(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "```go\nvar 中文 = 123 // hello\n\n```"
	a.prefs.Appearance.CodeTheme = "soft"
	tt := ui.NewTester(a.view, 400, 540)
	tt.Frame()
	a.mdActive = -1
	tt.Frame()
	blk := a.mdBlocks()[0]
	entry := a.previewBlock(blk)
	if !entry.codeReady {
		t.Fatal("真实预览未走代码缓存")
	}
	for _, dark := range []bool{false, true, false} {
		tt.SetDark(dark)
		for _, id := range []string{"soft", "vivid", "", "qoder"} {
			a.prefs.Appearance.CodeTheme = id
			tt.Frame()
			if id == "" {
				continue
			}
			cp := codeSchemes[id][0]
			if dark {
				cp = codeSchemes[id][1]
			}
			entry = a.previewBlock(blk)
			if entry.palette != cp {
				t.Fatalf("主题/配色未失效: %s dark=%v", id, dark)
			}
			want := codeLineSpans(blk.lines[0], codehl.Line(codehl.Lang(blk.info), blk.lines[0]), cp, ui.Color{})
			if !reflect.DeepEqual(entry.code[0], want) {
				t.Fatal("代码颜色或 token 不同")
			}
			if !reflect.DeepEqual(entry.code[1], []ui.Span{{Text: " "}}) {
				t.Fatal("空代码行布局变化")
			}
		}
	}
	a.tabs[0].Text = "```js\nconst a = 42;\n```"
	tt.Frame()
	newBlock := a.mdBlocks()[0]
	if a.previewBlock(newBlock) == entry || !a.previewBlock(newBlock).codeReady {
		t.Fatal("代码内容/语言更新后缓存错误")
	}
}

func TestPreviewTableCacheEditingSelection(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "| 甲 | 乙 |\n| --- | --- |\n| 中文😀 | **粗体** |"
	tt := ui.NewTester(a.view, 400, 540)
	tt.Frame()
	a.mdActive = -1
	tt.Frame()
	blk := a.mdBlocks()[0]
	entry := a.previewTable(blk)
	if !reflect.DeepEqual(entry.positions, tableCellPositions(blk.raw)) {
		t.Fatal("单元格偏移不同")
	}
	for row, cells := range blk.table.rows {
		for col, cell := range cells {
			want := projectRegion(cell.text, 0, utf8.RuneCountInString(cell.text), true, true)
			if !reflect.DeepEqual(entry.cells[row][col], want) {
				t.Fatal("单元格投影不同")
			}
		}
	}
	if err := tt.Click("单元格 2,1"); err != nil {
		t.Fatal(err)
	}
	tt.Type("新|值")
	if !strings.Contains(a.tabs[0].Text, `中文😀新\|值`) {
		t.Fatalf("缓存单元格编辑丢失: %q", a.tabs[0].Text)
	}
	if a.previewTable(a.mdBlocks()[0]) == entry {
		t.Fatal("编辑后复用过期表格")
	}
	tt.Key(0, ui.KeyTab)
	tt.Type("替换")
	if got := a.mdBlocks()[0].table.rows[1][1].text; got != "**替换**" {
		t.Fatalf("选区替换失败: %q", got)
	}
	tt.Key(ui.Shift, ui.KeyTab)
	if a.mdVisible != "中文😀新|值" {
		t.Fatalf("偏移映射失败: %q", a.mdVisible)
	}
	for range 3 {
		tt.Frame()
	}
	if !tt.Focused("内容") {
		t.Fatal("重复帧失焦")
	}
}
