package app

import (
	"github.com/egoist/mygo/ui"

	"zakki/internal/prefs"
	"zakki/locales"
)

// windowSizeRows 默认窗口大小：每行字数 × 行数，下方小字给出按当前字号换算的像素。
func (a *app) windowSizeRows(c *ui.Context) {
	adv := &a.settingsUI.draft.Advanced
	cols, rows := float64(adv.WindowColumns), float64(adv.WindowRows)
	settingsRow(c, locales.DefaultWindowColumns(), locales.DefaultWindowSizeHint(), func() {
		if ui.NumberInput(c.Key("window-columns"), &cols, prefs.MinWindowColumns, prefs.MaxWindowColumns, 1).
			Label(locales.DefaultWindowColumns()).Changed() {
			adv.WindowColumns = int(cols)
			_ = a.updatePreferences(func(p *preferences) { p.Advanced.WindowColumns = int(cols) })
		}
	})
	settingsRow(c, locales.DefaultWindowRows(), "", func() {
		if ui.NumberInput(c.Key("window-rows"), &rows, prefs.MinWindowRows, prefs.MaxWindowRows, 1).
			Label(locales.DefaultWindowRows()).Changed() {
			adv.WindowRows = int(rows)
			_ = a.updatePreferences(func(p *preferences) { p.Advanced.WindowRows = int(rows) })
		}
	})
	p := a.prefs
	p.Advanced = *adv
	p.Appearance = a.settingsUI.draft.Appearance
	w, h := defaultWindowSize(p)
	ui.Row(c).FillWidth().Padding(0, 0, 10, 0).Children(func() {
		ui.Box(c).Grow(1)
		ui.Text(c, locales.DefaultWindowPixels(w, h)).FontSize(11).TextColor(tMuted(c))
	})
}
