package app

import (
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"zakki/internal/codehl"
)

const livePreviewCacheLimit = 64

// 只保留当前块代际的少量最近使用块；投影与 token 不持有构建期 UI 对象。
type livePreviewCache struct {
	tabID  uint64
	blocks map[int]*livePreviewBlock
}

type livePreviewBlock struct {
	raw, info  string
	kind       blockKind
	projection *liveProjection
	positions  []tableCell
	cells      [][]liveProjection
	activeCell *liveProjection
	codeReady  bool
	palette    codePalette
	code       [][]ui.Span
}

func (a *app) previewBlock(blk liveBlock) *livePreviewBlock {
	cache := &a.previewCache
	id := a.activeTab().ID
	if cache.tabID != id {
		*cache = livePreviewCache{tabID: id}
	}
	if p := cache.blocks[blk.start]; p != nil && p.raw == blk.raw && p.kind == blk.kind && p.info == blk.info {
		return p
	}
	if cache.blocks == nil || len(cache.blocks) >= livePreviewCacheLimit {
		cache.blocks = make(map[int]*livePreviewBlock)
	}
	p := &livePreviewBlock{raw: blk.raw, info: blk.info, kind: blk.kind}
	cache.blocks[blk.start] = p
	return p
}

func (a *app) previewProjection(blk liveBlock) liveProjection {
	p := a.previewBlock(blk)
	if p.projection == nil {
		projection := projectBlock(blk)
		p.projection = &projection
	}
	return *p.projection
}

func (a *app) previewTable(blk liveBlock) *livePreviewBlock {
	p := a.previewBlock(blk)
	if p.cells == nil {
		p.positions = tableCellPositions(blk.raw)
		p.cells = make([][]liveProjection, len(blk.table.rows))
		for row, cells := range blk.table.rows {
			p.cells[row] = make([]liveProjection, len(cells))
			for col, cell := range cells {
				p.cells[row][col] = projectRegion(cell.text, 0, utf8.RuneCountInString(cell.text), true, true)
			}
		}
	}
	return p
}

func (p *livePreviewBlock) tableEditor(cell tableCell) liveProjection {
	if p.activeCell == nil || p.activeCell.start != cell.start || p.activeCell.end != cell.end {
		projection := projectRegion(p.raw, cell.start, cell.end, true, true)
		p.activeCell = &projection
	}
	return *p.activeCell
}

func (a *app) previewCode(blk liveBlock, cp codePalette) [][]ui.Span {
	p := a.previewBlock(blk)
	// 仅缓存显式 token 颜色；字体、继承文字色及布局仍逐帧从当前主题取值。
	if !p.codeReady || p.palette != cp {
		p.code = make([][]ui.Span, max(1, len(blk.lines)))
		lang := codehl.Lang(blk.info)
		for i := range p.code {
			line := " "
			if i < len(blk.lines) && blk.lines[i] != "" {
				line = blk.lines[i]
			}
			p.code[i] = codeLineSpans(line, codehl.Line(lang, line), cp, ui.Color{})
		}
		p.palette, p.codeReady = cp, true
	}
	return p.code
}
