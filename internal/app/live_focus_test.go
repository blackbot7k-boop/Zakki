package app

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// 滚动不改变焦点块；点在正文下方聚焦文末；点在某行旁边的留白聚焦那一行。
func TestLivePaneResizeAndScrollFocus(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = strings.Repeat("段落\n\n", 40) + "末尾"
	a.resyncLive(0)
	tt := ui.NewTester(a.view, 400, 540)
	if err := tt.Click("标签 1"); err != nil {
		t.Fatal(err)
	}
	tt.Scroll(200, 300, 0, 10000)
	if a.mdActive != 0 {
		t.Fatal("scrolling should not change the editing block")
	}
	// 点击文档下方的空白内边距后可以继续在文末输入。
	tt.ClickAt(200, 500)
	if a.mdActive != len(a.mdBlocks())-1 || !tt.Focused("内容") {
		t.Fatalf("blank click should focus document end: active=%d focused=%v", a.mdActive, tt.Focused("内容"))
	}
	tt.Type("X")
	if !strings.HasSuffix(a.tabs[0].Text, "末尾X") {
		t.Fatal("blank click must allow typing at document end")
	}
	for _, height := range []int{900, 340, 540} {
		tt.SetSize(400, height)
		tt.Key(ui.Cmd, ui.KeyF)
		tt.ClickAt(8, float32(height-60)) // 左侧留白：对上哪一行就编辑哪一行
		tt.Frame()                        // 虚拟列表：焦点编辑器所在行在点击后的下一帧构建
		if !tt.Focused("内容") || a.mdActive < 0 {
			t.Fatalf("left gutter after resize %d should focus the aligned line: active=%d focused=%v", height, a.mdActive, tt.Focused("内容"))
		}
	}
	// 点击最后一个块进入编辑（显式意图），光标在块尾，可继续输入。
	a.focusBlock(len(a.mdBlocks())-1, -1, -1)
	tt.Frame()
	if !tt.Focused("内容") || a.mdDraft != "末尾X" {
		t.Fatalf("block click should focus its editor: draft=%q", a.mdDraft)
	}
	// 点击已经聚焦的输入框不能被外层 Scroll 当成空白点击而卸载。
	if err := tt.Click("内容"); err != nil {
		t.Fatal(err)
	}
	if a.mdActive != len(a.mdBlocks())-1 || !tt.Focused("内容") {
		t.Fatalf("点击焦点输入框后不应退出编辑：active=%d focused=%v", a.mdActive, tt.Focused("内容"))
	}
	tt.Type("Y")
	if !strings.HasSuffix(a.tabs[0].Text, "末尾XY") {
		t.Fatalf("点击焦点输入框后输入丢失：%q", a.tabs[0].Text)
	}
}

func TestEmptyDotContentAreaFocusesEditor(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "第一点"
	for i := 1; i < len(a.tabs); i++ {
		a.tabs[i].Text = ""
	}
	tt := ui.NewTester(a.view, 400, 540)
	tt.Frame()

	for i := 1; i < len(a.tabs); i++ {
		if err := tt.Click(fmt.Sprintf("标签 %d", i+1)); err != nil {
			t.Fatal(err)
		}
		if a.active != i || a.mdActive != -1 || tt.Focused("内容") {
			t.Fatalf("switch to tab %d: active=%d block=%d focused=%v", i+1, a.active, a.mdActive, tt.Focused("内容"))
		}
		// 空点没有文字节点，点击内容区中部、底部或侧边均可输入。
		height := []int{540, 340, 900}[i%3]
		tt.SetSize(400, height)
		x := float32(200)
		if i%2 == 0 {
			x = 8
		}
		tt.ClickAt(x, float32(height-60))
		if a.mdActive != 0 || !tt.Focused("内容") {
			t.Fatalf("dot %d blank click: active=%d focused=%v", i+1, a.mdActive, tt.Focused("内容"))
		}
		tt.Type("新内容")
		if a.tabs[i].Text != "新内容" || a.tabs[0].Text != "第一点" {
			t.Fatalf("typing into dot %d: current=%q first=%q", i+1, a.tabs[i].Text, a.tabs[0].Text)
		}
		// 有内容后，点击空白仍可继续输入，不需要点中文字。
		tt.ClickAt(8, float32(height-60))
		if a.mdActive != 0 || !tt.Focused("内容") {
			t.Fatal("populated dot blank click should keep editor focused")
		}
		tt.Type("X")
		if a.tabs[i].Text != "新内容X" || !tt.Focused("内容") {
			t.Fatalf("dot %d populated click: text=%q focused=%v", i+1, a.tabs[i].Text, tt.Focused("内容"))
		}
	}
}

// 五行叠在同一段里时，点第 3 行右侧空白应插入在该行末尾。
func TestClickWrappedLineTailFocusesThatLine(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.mdActive = -1
	const doc = "一行\n二行\n三行\n四行\n五行"
	a.tabs[0].Text = doc
	tt := ui.NewTester(a.view, 420, 640)
	r, ok := tt.Find(doc)
	if !ok {
		t.Fatalf("找不到五行段落，texts %q", tt.Texts())
	}
	lineH := r.H / 5
	tt.ClickAt(r.X+r.W-8, r.Y+lineH*2+lineH/2)
	tt.Type("X")
	if got := a.tabs[0].Text; got != "一行\n二行\n三行X\n四行\n五行" {
		t.Fatalf("点第3行行尾应插入在该行，got %q active=%d", got, a.mdActive)
	}
}

// 各自成段的短行：行内右侧空白和页边留白都落在被点的那一行，纸面底部仍去文末。
func TestClickSeparateLineTailFocusesThatLine(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.mdActive = -1
	a.tabs[0].Text = "一行\n\n二行\n\n三行\n\n四行\n\n五行"
	tt := ui.NewTester(a.view, 420, 640)
	r, ok := tt.Find("三行")
	if !ok {
		t.Fatalf("找不到第三行，texts %q", tt.Texts())
	}
	tt.ClickAt(r.X+r.W-4, r.Y+r.H/2)
	tt.Type("X")
	if got := a.tabs[0].Text; got != "一行\n\n二行\n\n三行X\n\n四行\n\n五行" {
		t.Fatalf("点第三行行内空白应插入在该行，got %q", got)
	}

	a.mdActive = -1
	a.tabs[0].Text = "一行\n\n二行\n\n三行\n\n四行\n\n五行"
	a.invalidateMdCache()
	tt.Frame()
	r, ok = tt.Find("三行")
	if !ok {
		t.Fatal("重设后找不到第三行")
	}
	tt.ClickAt(416, r.Y+r.H/2)
	tt.Type("Y")
	if got := a.tabs[0].Text; got != "一行\n\n二行\n\n三行Y\n\n四行\n\n五行" {
		t.Fatalf("点第三行页边空白应插入在该行，got %q active=%d", got, a.mdActive)
	}

	end, ok := tt.Find("五行")
	if !ok {
		t.Fatal("找不到末行")
	}
	tt.ClickAt(200, end.Y+end.H+80)
	tt.Type("Z")
	if got := a.tabs[0].Text; got != "一行\n\n二行\n\n三行Y\n\n四行\n\n五行Z" {
		t.Fatalf("点纸面底部应续写在文末，got %q", got)
	}
}

// TestLivePanePreviewByDefault 启动即预览态：点击内容区之前，键盘输入不动文档。
func TestLeftGutterClickFocusesAlignedLine(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.mdActive = -1
	a.tabs[0].Text = "首段\n\n末段"
	tt := ui.NewTester(a.view, 400, 540)
	tt.ClickAt(8, 80)
	tt.Type("X")
	if a.tabs[0].Text != "X首段\n\n末段" {
		t.Fatalf("左侧留白应对准首段开头，got %q active=%d", a.tabs[0].Text, a.mdActive)
	}
}

func TestPopulatedDotBlankAreaFocusesDocumentEnd(t *testing.T) {
	for _, pos := range [][2]float32{{200, 300}, {200, 480}, {392, 300}} {
		t.Run(fmt.Sprint(pos), func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			a.mdActive = -1
			a.tabs[0].Text = "首段\n\n末段"
			tt := ui.NewTester(a.view, 400, 540)
			tt.ClickAt(pos[0], pos[1])
			if a.mdActive != len(a.mdBlocks())-1 || !tt.Focused("内容") {
				t.Fatalf("blank area should focus last block: active=%d focused=%v", a.mdActive, tt.Focused("内容"))
			}
			tt.Type("X")
			if a.tabs[0].Text != "首段\n\n末段X" {
				t.Fatalf("blank area typing should append at document end: %q", a.tabs[0].Text)
			}
			// 点击其他段落不能被视口兜底重新定向到文末。
			if err := tt.Click("首段"); err != nil {
				t.Fatal(err)
			}
			tt.Type("Y")
			if a.tabs[0].Text != "首段Y\n\n末段X" {
				t.Fatalf("text click should edit the clicked block: %q", a.tabs[0].Text)
			}
		})
	}
}

// TestLivePanePreviewByDefault 启动即预览态：点击内容区之前，键盘输入不动文档。
func TestLivePanePreviewByDefault(t *testing.T) {
	// newTestApp 用临时目录；newApp 会指向真实用户数据目录，跑到这里会把
	// 用户的偏好与笔记覆盖成测试内容。生产启动态是 mdActive = -1，手动设回去。
	a := newTestApp(t)
	a.mdActive = -1
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "正文"
	tt := ui.NewTester(a.view, 400, 540)
	tt.Frame()
	if a.mdActive != -1 {
		t.Fatalf("app should boot in preview mode: active=%d", a.mdActive)
	}
	tt.Type("X")
	if a.tabs[0].Text != "正文" {
		t.Fatalf("preview mode must be read-only: %q", a.tabs[0].Text)
	}
	// 点第二个块（若有渲染文本可寻址）进入编辑——这里用函数级定位保证契约质量。
	a.focusBlock(0, utf8.RuneCountInString(a.tabs[0].Text), utf8.RuneCountInString(a.tabs[0].Text)) // 块尾
	tt.Frame()
	if !tt.Focused("内容") {
		t.Fatal("explicit block click must enter its editor")
	}
	tt.Type("X")
	if a.tabs[0].Text != "正文X" {
		t.Fatalf("edit after explicit click: %q", a.tabs[0].Text)
	}
	// 显式退出编辑仍可退回预览。
	a.blurLive()
	tt.Frame()
	if a.mdActive != -1 || tt.Focused("内容") {
		t.Fatal("blur must return to preview")
	}
}
