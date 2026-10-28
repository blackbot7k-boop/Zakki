package app

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTimeProjectionAtomicObject(t *testing.T) {
	for _, raw := range []string{"交报告 @2026-11-01 10:00 ~1h", "交报告 @2026-11-01 10:00 ~1h 附表", "@2026-11-01 10:00 ~1h 和张三确认合同", "- [ ] **交报告** @2026-11-01 10:00 ~1h 附上[预算表](https://example.com)", "前言\n交报告 @2026-11-01 10:00 ~1h 附表"} {
		t.Run(raw, func(t *testing.T) {
			p := projectBlock(liveBlocks(raw)[0])
			if len(p.objects) != 1 || strings.Contains(p.text, "@2026") || strings.Count(p.text, string(timeObjectRune)) != 1 {
				t.Fatalf("时间必须是一个对象，而不是展开源码: %+v", p)
			}
			at := p.objects[0]
			u := p.units[at]
			if source := string([]rune(raw)[u.start:u.end]); source != "@2026-11-01 10:00 ~1h" {
				t.Fatalf("对象必须映射完整时间与提醒: %q", source)
			}
			if got := p.fragment(0, len(p.units)); got != raw[runeToByte(raw, p.start):] {
				t.Fatalf("可见片段必须无损还原源码: %q", got)
			}
			if s, e := p.visibleSelection(u.start+1, u.end-1); s != at || e != at+1 {
				t.Fatalf("命中部分源码时必须选中整组件: %d..%d", s, e)
			}
			next := p.text + "继续"
			got, caret := p.replace(next, utf8.RuneCountInString(next))
			want := raw + "继续"
			expectedCaret := utf8.RuneCountInString(next)
			if strings.HasSuffix(raw, "~1h") {
				want = raw + " 继续"
				expectedCaret++
			}
			if strings.HasSuffix(raw, "(https://example.com)") {
				want = strings.Replace(raw, "[预算表]", "[预算表继续]", 1)
			}
			updated := projectBlock(liveBlocks(got)[0])
			if got != want || updated.visibleOffset(caret) != expectedCaret {
				t.Fatalf("组件后输入应保留顺序、原有行内格式和末尾光标: raw=%q caret=%d", got, caret)
			}
			without := string([]rune(p.text)[:at]) + string([]rune(p.text)[at+1:])
			deleted, _ := p.replaceSelection(without, at, at, at+1)
			if strings.Contains(deleted, "@2026") || strings.ContainsRune(deleted, timeObjectRune) {
				t.Fatalf("删除对象应完整删除源码，不泄漏占位符: %q", deleted)
			}
		})
	}
}
