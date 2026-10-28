package app

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"zakki/internal/markdown"
	"zakki/locales"
)

type timeRecurDraft struct {
	preset   string
	every    float64
	unit     string
	weekdays [7]bool
	hasEnd   bool
	end      time.Time
	original string
}

type timeRecurOption struct{ value, label string }

func timeRecurPresets() []timeRecurOption {
	return []timeRecurOption{
		{"none", locales.TimeRecurNone()},
		{"^1d", locales.TimeRecurDaily()},
		{"^w@1-5", locales.TimeRecurWorkdays()},
		{"^1w", locales.TimeRecurWeekly()},
		{"^1mo", locales.TimeRecurMonthly()},
		{"^1y", locales.TimeRecurYearly()},
		{"custom", locales.TimeRecurCustom()},
	}
}

func newTimeRecurDraft(raw string, due time.Time) timeRecurDraft {
	r := timeRecurDraft{preset: "none", every: 1, unit: "d", end: due, original: raw}
	if raw == "" {
		return r
	}
	value, err := markdown.ParseRule(raw)
	var rule markdown.Rule
	if err != nil || json.Unmarshal([]byte(value), &rule) != nil {
		return r
	}
	r.preset = "custom"
	r.every, r.unit = float64(rule.Every), rule.Unit
	for _, day := range rule.Weekdays {
		r.weekdays[day-1] = true
	}
	if rule.End != "" {
		r.hasEnd = true
		r.end, _ = time.ParseInLocation("2006-01-02", rule.End, due.Location())
	}
	for _, preset := range timeRecurPresets() {
		if preset.value == "none" || preset.value == "custom" {
			continue
		}
		candidate, _ := markdown.ParseRule(preset.value)
		if candidate == value {
			r.preset = preset.value
			break
		}
	}
	return r
}

func (r timeRecurDraft) token() string {
	if r.preset == "none" || r.preset == "" {
		return ""
	}
	token := r.preset
	if r.preset == "custom" {
		token = "^" + strconv.Itoa(clamp(int(r.every), 1, 999)) + r.unit
		if r.unit == "w" && int(r.every) == 1 {
			var days []string
			for i, selected := range r.weekdays {
				if selected {
					days = append(days, strconv.Itoa(i+1))
				}
			}
			if len(days) > 0 {
				token += "@" + strings.Join(days, ",")
			}
		}
		if r.hasEnd {
			token += "<" + r.end.Format("2006-01-02")
		}
	}
	value, err := markdown.ParseRule(token)
	if err != nil {
		return ""
	}
	original, err := markdown.ParseRule(r.original)
	if err == nil && original == value {
		return r.original
	}
	return token
}

// timeRecurSelect 持久绑定规则值，文案在每帧本地化；嵌套弹层限高。
func timeRecurSelect(c *ui.Context, value *string, options []timeRecurOption) ui.Element {
	sel := ui.SelectBase(c, value)
	label := ""
	for _, option := range options {
		if option.value == *value {
			label = option.label
			break
		}
	}
	selectTrigger(c, sel.Trigger, func() { ui.Text(c, label).SingleLine() })
	sel.Trigger.MinWidth(0).FillWidth()
	sel.Popup(func(panel ui.Element) {
		selectPanel(c, panel)
		ui.Scroll(c).FillWidth().MaxHeight(240).Children(func() {
			ui.Column(c).FillWidth().Children(func() {
				for _, option := range options {
					item := sel.Item(option.value).FillWidth().Padding(6, 10).Radius(6)
					if item.Highlighted() {
						item.Background(c.Theme().Accent).TextColor(c.Theme().AccentText)
					}
					item.Children(func() { ui.Text(c, option.label).SingleLine() })
				}
			})
		})
	})
	return sel.Trigger
}

func (a *app) timeRecurContent(c *ui.Context) {
	r := &a.timePop.recur
	section := ui.Column(c.Key("time-recur-section")).Gap(8).FillWidth()
	section.Children(func() {
		ui.Text(c, locales.TimePopRecur()).FontSize(11.5).TextColor(c.Theme().TextMuted)
		timeRecurSelect(c.Key("time-recur-preset"), &r.preset, timeRecurPresets()).Disabled(!a.timePop.hasTime)
		if r.preset != "custom" {
			return
		}
		ui.Row(c.Key("time-recur-interval")).Gap(8).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.NumberInput(c.Key("time-recur-every"), &r.every, 1, 999, 1).Label(locales.TimeRecurInterval())
			ui.Box(c).Grow(1).MinWidth(0).Children(func() {
				timeRecurSelect(c.Key("time-recur-unit"), &r.unit, []timeRecurOption{{"d", locales.TimeRecurDays()}, {"w", locales.TimeRecurWeeks()}, {"mo", locales.TimeRecurMonths()}, {"y", locales.TimeRecurYears()}})
			})
		})
		if r.unit == "w" && int(r.every) == 1 {
			ui.Row(c.Key("time-recur-weekdays")).Gap(4).FillWidth().Wrap().Children(func() {
				for i := range 7 {
					day := i
					label := timeChipWeekdayName(time.Weekday((i + 1) % 7))
					button := ui.ButtonBase(c.Key("time-recur-day-"+strconv.Itoa(i))).Padding(4, 7).Radius(6).FocusRing(true).Cursor(ui.CursorPointer)
					if r.weekdays[i] {
						button.Background(c.Theme().Accent).TextColor(c.Theme().AccentText)
					} else {
						button.Background(c.Theme().Surface)
					}
					button.Children(func() { ui.Text(c, label).FontSize(11) })
					button.OnClick(func() { r.weekdays[day] = !r.weekdays[day] })
				}
			})
		}
		ui.Row(c.Key("time-recur-end")).Gap(8).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Checkbox(c.Key("time-recur-has-end"), &r.hasEnd, locales.TimeRecurEnd())
			if r.hasEnd {
				ui.DateInput(c.Key("time-recur-end-date"), &r.end).Grow(1).MinWidth(0)
			}
		})
	})
}
