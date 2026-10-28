package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"zakki/internal/markdown"
	"zakki/locales"
)

func TestTimeRecurDraftRoundTrip(t *testing.T) {
	due := time.Date(2026, 11, 1, 10, 5, 0, 0, time.Local)
	for _, tc := range []struct{ raw, preset string }{
		{"", "none"}, {"^1d", "^1d"}, {"^d", "^1d"},
		{"^w@1-5", "^w@1-5"}, {"^1w@5,3,1-4", "^w@1-5"},
		{"^1w", "^1w"}, {"^mo", "^1mo"}, {"^1y", "^1y"},
		{"^999d", "custom"}, {"^2w", "custom"}, {"^3mo", "custom"},
		{"^w@7,1,3<2027-02-28", "custom"}, {"^1d<2026-11-30", "custom"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			r := newTimeRecurDraft(tc.raw, due)
			if r.preset != tc.preset || r.token() != tc.raw {
				t.Fatalf("草稿 = %+v，写回 %q，期望 preset=%q 原样写回 %q", r, r.token(), tc.preset, tc.raw)
			}
			if r.hasEnd && r.end.Location() != due.Location() {
				t.Fatalf("结束日期应沿用锚点时区: %s", r.end)
			}
		})
	}
}

func newTimeRecurTester(t *testing.T, raw, locale string, anchorY float32) (*app, *ui.Tester, *ui.Rect) {
	t.Helper()
	i18n.SetLocaleFunc(func() string { return locale })
	t.Cleanup(func() { i18n.SetLocaleFunc(mygo.App.Locale) })
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = raw
	blk := a.mdBlocks()[0]
	a.openTimePopover(blk)
	bounds := new(ui.Rect)
	tt := ui.NewTester(func(c *ui.Context) {
		anchor := ui.Row(c.Key("recur-anchor")).FillWidth().Height(28).Top(anchorY)
		a.timePopover(c, anchor, blk).DrawOver(func(_ *ui.Painter, r ui.Rect) { *bounds = r })
	}, 440, 600)
	tt.SetLocale(locale)
	return a, tt, bounds
}

func clickTimeRecurText(t *testing.T, tt *ui.Tester, text string) {
	t.Helper()
	if err := tt.Click(text); err != nil {
		t.Fatal(err)
	}
}

func TestTimeRecurPresetSelectionAndClear(t *testing.T) {
	for _, locale := range []string{"zh-Hans", "en-US"} {
		for index := range timeRecurPresets()[:6] {
			t.Run(locale+"/"+timeRecurPresets()[index].value, func(t *testing.T) {
				const raw = "正文\n续行 @2026-11-01 10:05 ~10m ^w@1,3,5<2027-02-28 后文"
				a, tt, _ := newTimeRecurTester(t, raw, locale, 0)
				option := timeRecurPresets()[index]
				clickTimeRecurText(t, tt, locales.TimeRecurCustom())
				clickTimeRecurText(t, tt, option.label)
				if !a.timePop.open || a.timePop.recur.preset != option.value {
					t.Fatalf("嵌套选择未写入草稿: open=%v draft=%+v texts=%q", a.timePop.open, a.timePop.recur, tt.Texts())
				}
				clickTimeRecurText(t, tt, locales.TimePopDone())
				want := "正文\n续行 @2026-11-01 10:05 ~10m"
				if option.value != "none" {
					want += " " + option.value
				}
				want += " 后文"
				if a.timePop.open || a.tabs[0].Text != want {
					t.Fatalf("选择后应保留日期、提醒、软换行及正文: open=%v got %q want %q", a.timePop.open, a.tabs[0].Text, want)
				}
				a.undoLive(false)
				if a.tabs[0].Text != raw {
					t.Fatalf("撤销未恢复原规则: %q", a.tabs[0].Text)
				}
				a.undoLive(true)
				if a.tabs[0].Text != want {
					t.Fatalf("重做未恢复所选规则: %q", a.tabs[0].Text)
				}
			})
		}
	}
}

func TestTimeRecurPresetThenCustomKeepsSelectedRule(t *testing.T) {
	for _, tc := range []struct{ raw, preset string }{
		{"", "^w@1-5"},
		{"^2w<2027-02-28", "^1mo"},
	} {
		t.Run(tc.preset, func(t *testing.T) {
			raw := "任务 @2026-11-01 10:05"
			if tc.raw != "" {
				raw += " " + tc.raw
			}
			a, tt, _ := newTimeRecurTester(t, raw, "zh-Hans", 0)
			current := locales.TimeRecurNone()
			if tc.raw != "" {
				current = locales.TimeRecurCustom()
			}
			var selected string
			for _, option := range timeRecurPresets() {
				if option.value == tc.preset {
					selected = option.label
				}
			}
			clickTimeRecurText(t, tt, current)
			clickTimeRecurText(t, tt, selected)
			clickTimeRecurText(t, tt, selected)
			clickTimeRecurText(t, tt, locales.TimeRecurCustom())
			got := a.timePop.recur.token()
			gotRule, gotErr := markdown.ParseRule(got)
			wantRule, wantErr := markdown.ParseRule(tc.preset)
			if gotErr != nil || wantErr != nil || gotRule != wantRule {
				t.Fatalf("自定义应从刚选择的预设继续编辑: got %q want %q draft=%+v", got, tc.preset, a.timePop.recur)
			}
			clickTimeRecurText(t, tt, locales.TimePopDone())
			if a.timePop.open || a.tabs[0].Text != "任务 @2026-11-01 10:05 "+got {
				t.Fatalf("进入自定义但不修改时应保持所选规则: %q", a.tabs[0].Text)
			}
		})
	}
}

func TestTimeRecurNoChangePreservesSource(t *testing.T) {
	for _, recur := range []string{"^d", "^1w@5,3,1-4", "^2w<2027-02-28", "^w@7,1,3<2027-02-28"} {
		t.Run(recur, func(t *testing.T) {
			raw := "任务 @2026-11-01 10:05 ~1h " + recur + " 备注"
			a, tt, _ := newTimeRecurTester(t, raw, "zh-Hans", 0)
			clickTimeRecurText(t, tt, locales.TimePopDone())
			if a.timePop.open || a.tabs[0].Text != raw {
				t.Fatalf("未编辑时应原样保留规则: open=%v got %q want %q", a.timePop.open, a.tabs[0].Text, raw)
			}
		})
	}
}

func TestTimeRecurCustomControls(t *testing.T) {
	for _, locale := range []string{"zh-Hans", "en-US"} {
		t.Run(locale, func(t *testing.T) {
			a, tt, _ := newTimeRecurTester(t, "任务 @2026-11-01 10:05", locale, 0)
			clickTimeRecurText(t, tt, locales.TimeRecurNone())
			clickTimeRecurText(t, tt, locales.TimeRecurCustom())
			clickTimeRecurText(t, tt, locales.TimeRecurDays())
			clickTimeRecurText(t, tt, locales.TimeRecurWeeks())
			monday := timeChipWeekdayName(time.Monday)
			friday := timeChipWeekdayName(time.Friday)
			clickTimeRecurText(t, tt, monday)
			clickTimeRecurText(t, tt, friday)
			if got := a.timePop.recur.token(); got != "^1w@1,5" {
				t.Fatalf("自定义周几未生效: %q", got)
			}
			clickTimeRecurText(t, tt, "+")
			if a.timePop.recur.every != 2 || tt.HasText(monday) || a.timePop.recur.token() != "^2w" {
				t.Fatalf("间隔大于一周应停用周几: draft=%+v texts=%q", a.timePop.recur, tt.Texts())
			}
			clickTimeRecurText(t, tt, "−")
			if !tt.HasText(monday) || a.timePop.recur.token() != "^1w@1,5" {
				t.Fatalf("恢复每周应保留周几草稿: %+v", a.timePop.recur)
			}
			clickTimeRecurText(t, tt, locales.TimeRecurWeeks())
			clickTimeRecurText(t, tt, locales.TimeRecurMonths())
			_, weekdaysVisible := tt.Find(monday)
			if weekdaysVisible || a.timePop.recur.token() != "^1mo" {
				t.Fatalf("月规则不能携带周几: %+v", a.timePop.recur)
			}
			clickTimeRecurText(t, tt, locales.TimeRecurEnd())
			if !a.timePop.recur.hasEnd || a.timePop.recur.token() != "^1mo<2026-11-01" {
				t.Fatalf("勾选结束日期应使用草稿日期: %+v", a.timePop.recur)
			}
			clickTimeRecurText(t, tt, locales.TimeRecurEnd())
			clickTimeRecurText(t, tt, locales.TimePopDone())
			if a.timePop.open || a.tabs[0].Text != "任务 @2026-11-01 10:05 ^1mo" {
				t.Fatalf("取消结束日期后应写回无 end 的自定义规则: %q", a.tabs[0].Text)
			}
		})
	}
}

func TestTimeRecurIntervalBounds(t *testing.T) {
	a, tt, _ := newTimeRecurTester(t, "任务 @2026-11-01 10:05 ^2y", "zh-Hans", 0)
	field, ok := tt.Find(locales.TimeRecurInterval())
	if !ok {
		t.Fatalf("找不到重复间隔输入: %q", tt.Texts())
	}
	setInterval := func(text string) {
		t.Helper()
		tt.ClickAt(field.X+20, field.Y+field.H/2)
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type(text)
	}
	setInterval("999")
	if a.timePop.recur.every != 999 || a.timePop.recur.token() != "^999y" {
		t.Fatalf("间隔上限未生效: %+v", a.timePop.recur)
	}
	clickTimeRecurText(t, tt, "+")
	if a.timePop.recur.every != 999 {
		t.Fatalf("间隔越过上限: %v", a.timePop.recur.every)
	}
	setInterval("1000")
	if a.timePop.recur.every != 999 {
		t.Fatalf("非法输入覆盖了有效间隔: %v", a.timePop.recur.every)
	}
	setInterval("1")
	clickTimeRecurText(t, tt, "−")
	if a.timePop.recur.every != 1 {
		t.Fatalf("间隔越过下限: %v", a.timePop.recur.every)
	}
	setInterval("0")
	if a.timePop.recur.every != 1 {
		t.Fatalf("零间隔被接受: %v", a.timePop.recur.every)
	}
	clickTimeRecurText(t, tt, locales.TimePopDone())
	if a.timePop.open || a.tabs[0].Text != "任务 @2026-11-01 10:05 ^1y" {
		t.Fatalf("无效输入后应提交最后有效间隔: %q", a.tabs[0].Text)
	}
}

func TestTimeRecurDisabledWithoutTime(t *testing.T) {
	for _, locale := range []string{"zh-Hans", "en-US"} {
		t.Run(locale, func(t *testing.T) {
			a, tt, _ := newTimeRecurTester(t, "任务", locale, 0)
			clickTimeRecurText(t, tt, locales.TimeRecurNone())
			if tt.HasText(locales.TimeRecurDaily()) || a.timePop.recur.preset != "none" {
				t.Fatalf("未确认日期时间时重复选择器必须禁用: %+v texts=%q", a.timePop.recur, tt.Texts())
			}
			clickTimeRecurText(t, tt, locales.TimePopDone())
			clickTimeRecurText(t, tt, locales.TimePopClear())
			if !a.timePop.open || a.tabs[0].Text != "任务" {
				t.Fatalf("未确认日期时间时底部动作必须禁用: open=%v text=%q", a.timePop.open, a.tabs[0].Text)
			}
			dateLabel := i18n.FormatDate(locale, a.timePop.due, i18n.DateOptions{Skeleton: "yMd"})
			clickTimeRecurText(t, tt, dateLabel)
			tt.Key(0, ui.KeyRight)
			tt.Key(0, ui.KeyEnter)
			if !a.timePop.hasTime {
				t.Fatal("选择日期后应确认时间并启用重复")
			}
			clickTimeRecurText(t, tt, locales.TimeRecurNone())
			clickTimeRecurText(t, tt, locales.TimeRecurDaily())
			want := "任务 @" + a.timePop.due.Format("2006-01-02 15:04") + " ^1d"
			clickTimeRecurText(t, tt, locales.TimePopDone())
			if a.timePop.open || a.tabs[0].Text != want {
				t.Fatalf("启用后的重复规则应随首次时间提交: open=%v text=%q want=%q", a.timePop.open, a.tabs[0].Text, want)
			}
		})
	}
}

func TestTimeRecurFullViewSmallWindow(t *testing.T) {
	for _, locale := range []string{"zh-Hans", "en-US"} {
		t.Run(locale, func(t *testing.T) {
			i18n.SetLocaleFunc(func() string { return locale })
			t.Cleanup(func() { i18n.SetLocaleFunc(mygo.App.Locale) })
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			raw := "任务 @2026-11-01 10:05 ^w@1,3,5<2027-02-28 后文"
			a.tabs[0].Text = raw
			a.mdActive = -1
			a.focusBlock(0, -1, -1)
			a.openTimePopover(a.mdBlocks()[0])
			tt := ui.NewTester(a.view, 440, 600)
			tt.SetLocale(locale)
			for _, text := range []string{locales.TimeRecurCustom(), locales.TimeRecurEnd(), locales.TimePopClear(), locales.TimePopDone()} {
				r, ok := tt.Find(text)
				if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 440 || r.Y+r.H > 600 {
					t.Fatalf("主视图选择器内容 %q 超出小窗口: %+v texts=%q", text, r, tt.Texts())
				}
			}
			clickTimeRecurText(t, tt, locales.TimePopDone())
			if a.timePop.open || a.tabs[0].Text != raw {
				t.Fatalf("主视图完成必须一次生效且保持未修改规则: open=%v text=%q", a.timePop.open, a.tabs[0].Text)
			}
		})
	}
}

func TestTimeRecurCustomLayoutAndEndDate(t *testing.T) {
	for _, locale := range []string{"zh-Hans", "en-US"} {
		for _, anchorY := range []float32{0, 520} {
			t.Run(locale+"/"+timeWheelNumber(int(anchorY)), func(t *testing.T) {
				a, tt, bounds := newTimeRecurTester(t, "任务 @2026-11-01 10:05 ^w@1,3,5<2027-02-28", locale, anchorY)
				labels := []string{locales.TimePopDate(), locales.TimePopClock(), locales.TimePopAlert(), locales.TimePopRecur(), locales.TimeRecurInterval(), locales.TimeRecurWeeks(), locales.TimeRecurEnd(), locales.TimePopClear(), locales.TimePopDone()}
				for day := time.Sunday; day <= time.Saturday; day++ {
					labels = append(labels, timeChipWeekdayName(day))
				}
				for _, label := range labels {
					r, ok := tt.Find(label)
					if !ok || r.X < bounds.X || r.Y < bounds.Y || r.X+r.W > bounds.X+bounds.W || r.Y+r.H > bounds.Y+bounds.H || r.Y+r.H > 600 || r.Y < 0 {
						t.Fatalf("小窗口内容 %q 不可达: rect=%+v panel=%+v texts=%q", label, r, bounds, tt.Texts())
					}
				}
				if bounds.W != timePopoverPanelWidth {
					t.Fatalf("面板宽度应固定: %+v", bounds)
				}
				endLabel := i18n.FormatDate(locale, a.timePop.recur.end, i18n.DateOptions{Skeleton: "yMd"})
				clickTimeRecurText(t, tt, endLabel)
				if !a.timePop.open {
					t.Fatal("打开嵌套日历不应关闭时间面板")
				}
				nextDay := time.Date(2027, 3, 1, 0, 0, 0, 0, time.Local)
				clickTimeRecurText(t, tt, i18n.FormatDate(locale, nextDay, i18n.DateOptions{Skeleton: "yMMMMd"}))
				if got := a.timePop.recur.end.Format("2006-01-02"); got != "2027-03-01" {
					t.Fatalf("结束日期未更新: %s texts=%q", got, tt.Texts())
				}
				clickTimeRecurText(t, tt, locales.TimePopDone())
				if want := "任务 @2026-11-01 10:05 ^1w@1,3,5<2027-03-01"; a.timePop.open || a.tabs[0].Text != want {
					t.Fatalf("修改结束日期后一次完成应写回: open=%v got %q want %q", a.timePop.open, a.tabs[0].Text, want)
				}
			})
		}
	}
}
