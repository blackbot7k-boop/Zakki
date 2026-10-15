package app

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tot-md-mygo/internal/notes"
	iprefs "tot-md-mygo/internal/prefs"
)

// tabsState 标签数据与当前选中下标。
type tabsState struct {
	tabs      []tab
	active    int
	nextTabID uint64
}

// tabStripState 标签栏液态动画与重命名（主线程；详见 tab_strip.go/view.go）。
type tabStripState struct {
	morph            tabMorphState
	lastSlots        []tabSlotGeom
	lastBody         tabBody
	tabScroll        ui.ScrollState
	tabViewportWidth float32
	lastIDs          []uint64
	builtActiveID    uint64
	renaming         uint64
	renameDraft      string
	renameFocus      int
	renameComposing  bool
	animNow          func() time.Time
}

// findState 查找条与匹配跳转。
type findState struct {
	findOpen    bool
	query       string
	matchIdx    int
	jumpPending bool
}

// liveEditorState Typora 式实时预览编辑（live_edit.go / live_blocks.go）。
type liveEditorState struct {
	mdVisible          string
	mdProjection       liveProjection
	liveUndo, liveRedo []liveEdit
	liveUndoPending    int
	editorHandle       ui.Handle
	documentMenu       bool
	editMenuPending    string
	mdActive           int
	mdDraft            string
	mdCaret            int
	mdSelEnd           int
	keyHint            ui.Key
	hintArmed          bool
	mdComposing        bool
	prevComposing      bool
	tableBackward      bool
	tableUndoPending   bool
	tableTools         tableTools
	autoTableBefore    string
	autoTableAfter     string
	autoTableLine      int
	autoTableCol       int
	autoTableCaret     int
	liveFocus          int
	selSyncStart       int
	selSyncEnd         int
	// selDoc 文档级全选：Cmd+A 选中全部块后，复制/剪切/删除/输入按
	// 文档区间语义在 app 层拦截，不进单块控件。selDocStart/End 为文档 rune 偏移。
	selDoc                 bool
	selDocStart, selDocEnd int
	// clipSvc 缓存窗口服务：文档级复制/剪切写剪贴板用（headless 测试安全）。
	clipSvc          ui.Services
	clipSvcOK        bool
	scrollActive     bool
	liveClickHandled bool
	// liveScroll 实时预览滚动位置：手账纸面画在视口上，按它对齐内容滚动。
	liveScroll     ui.ScrollState
	mdCacheValid   bool
	mdCacheDoc     string
	mdCacheBlocks  []liveBlock
	mdCacheLines   []string
	mdCacheOffsets []int // 块首 rune 偏移（末位为全文宽度），见 rebuildRuneOffsets
	mdCacheWidths  []int // 每块含换行的 rune 宽度
	// 提交管线 hint：spliceDoc 刚拼好的新文档行切分，mdBlocks 直接复用免一次全文 Split。
	spliceHintDoc   string
	spliceHintLines []string
	// 全文派生量的缓存：键是文档串（与可选的查询串），每帧重建视图时避免全文扫描。
	statsCacheDoc     string
	statsCacheLines   int
	statsCacheWords   int
	statsCacheChars   int
	rangesCacheDoc    string
	rangesCacheQuery  string
	rangesCacheDark   bool
	rangesCache       []ui.TextRange
	matchesCacheDoc   string
	matchesCacheQuery string
	matchesCache      []ui.TextRange
	lowerDoc          string
	lowerCache        string
}

// formatState 格式命令意图与链接浮层（live_format.go）。
type formatState struct {
	mdPending        map[spanKind]bool
	lastPendingCaret int
	pendingBlock     int
	pendingEditKind  editKind
	nextEditIsPaste  bool
	linkTool         linkToolState
}

// sourceEditorState 源码模式选区与撤销快照。
type sourceEditorState struct {
	srcCaret        int
	srcSelEnd       int
	srcSelSyncStart int
	srcSelSyncEnd   int
	srcPrev         liveSnapshot
}

// desktopRefs 窗口、菜单、全局热键与状态栏提示。
type desktopRefs struct {
	win          *mygo.Window
	winOpen      bool
	hidden       bool
	menu         *mygo.Menu
	miPreview    *mygo.MenuItem
	miFind       *mygo.MenuItem
	miFindNext   *mygo.MenuItem
	miFindPrev   *mygo.MenuItem
	miTop        *mygo.MenuItem
	hotkeyAcc    string
	noticeHotkey string
	notice       string
}

// prefsRuntimeState 内存中的偏好及其一次性迁移结果。
type prefsRuntimeState struct {
	prefs          preferences
	prefsMigration migrationResult
}

// settingsWindowState 设置窗口与草稿 UI。
type settingsWindowState struct {
	settingsWin  *mygo.Window
	settingsPage string
	settingsUI   settingsUIState
}

// app 是窗口展示的全部状态。它只能在主线程被改动；
// 其他 goroutine 通过 Window.Update 回到主线程。
// 领域字段经匿名嵌入提升，调用方仍可用 a.mdActive 等形式访问。
type app struct {
	tabsState
	tabStripState
	findState
	liveEditorState
	formatState
	sourceEditorState
	desktopRefs
	settingsWindowState
	prefsRuntimeState
	onboarding onboardingState

	cfg   settings
	ready bool

	notes      *notes.Store
	now        func() time.Time
	prefsStore *iprefs.Store
}
