package main

import (
	"math"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	dotDiameter    float32 = 12
	dotGap         float32 = 12
	pillHeight     float32 = 22
	pillPadX       float32 = 11
	bridgeReach    float32 = 10
	pillTitleSize  float32 = 12
	morphPadX      float32 = 24
	tabTrackHeight float32 = 38
)

func clampF(v, lo, hi float32) float32 { return min(max(v, lo), hi) }

const defaultTabTitleWidth = float32(120)

func (a *app) tabTitleWidthLimit() float32 {
	w := a.cfg.TabTitleWidth
	if w <= 0 {
		w = defaultTabTitleWidth
	}
	return clampF(w, 80, 220)
}

func (a *app) tabAnimDuration() time.Duration {
	speed := a.cfg.TabAnimSpeed
	if speed <= 0 {
		speed = 1
	}
	return time.Duration(float32(time.Millisecond) * 420 / clampF(speed, 0.5, 2))
}

func (a *app) glassStrength() float32 {
	s := a.cfg.GlassStrength
	if s <= 0 {
		s = 0.5
	}
	return clampF(s, 0.2, 0.9)
}

type tabSlotGeom struct{ w, cx float32 }

func pillWidthOf(titleW float32) float32 { return pillHeight + max(0, titleW) }

func tabSlotWidths(capsuleW float32, n, active int) []float32 {
	widths := make([]float32, n)
	for i := range widths {
		widths[i] = dotDiameter
		if i == active {
			widths[i] = capsuleW
		}
	}
	return widths
}

func layoutTabSlots(widths []float32) ([]tabSlotGeom, float32) {
	return layoutTabSlotsWithGap(widths, dotGap)
}

func layoutTabSlotsWithGap(widths []float32, gap float32) ([]tabSlotGeom, float32) {
	slots := make([]tabSlotGeom, len(widths))
	var x float32
	for i, w := range widths {
		slots[i] = tabSlotGeom{w: w, cx: x + w/2}
		x += w + gap
	}
	if len(widths) > 0 {
		x -= gap
	}
	return slots, x
}

func fitTabLayout(room, titleW float32, n, active int) ([]tabSlotGeom, float32, float32) {
	gap := dotGap
	base := float32(n-1)*dotDiameter + pillHeight
	if n > 1 {
		gap = clampF((room-base-titleW)/float32(n-1), 4, dotGap)
	}
	titleW = min(titleW, max(0, room-base-float32(n-1)*gap))
	slots, total := layoutTabSlotsWithGap(tabSlotWidths(pillWidthOf(titleW), n, active), gap)
	return slots, total, titleW
}

func easeLiquid(t float32) float32 {
	t = clampF(t, 0, 1)
	u := t - 1
	return 1 + 2.2*u*u*u + 1.2*u*u
}

func interpSlots(from, to []tabSlotGeom, t float32) []tabSlotGeom {
	out := slices.Clone(to)
	for i := range out {
		if i < len(from) {
			out[i] = tabSlotGeom{w: from[i].w + (to[i].w-from[i].w)*t, cx: from[i].cx + (to[i].cx-from[i].cx)*t}
		}
	}
	return out
}

func bridgeWidthFor(gap float32) float32 {
	if gap >= bridgeReach {
		return 0
	}
	t := clampF(1-gap/bridgeReach, 0, 1)
	return t * t * pillHeight * 0.8
}

func edgeGap(pillCx, halfWidth, dotCx, dr float32) float32 {
	return float32(math.Abs(float64(pillCx-dotCx))) - halfWidth - dr
}

func titleAlphas(t float32) (float32, float32) {
	return clampF((0.35-t)/0.35, 0, 1), clampF((t-0.55)/0.45, 0, 1)
}

func pillTextColor(bg ui.Color) ui.Color {
	white, dark := ui.RGB(255, 255, 255), ui.RGB(28, 28, 30)
	if contrastRatio(white, bg) >= contrastRatio(dark, bg) {
		return white
	}
	return dark
}

func emptyDotInk(dark bool) ui.Color {
	if dark {
		return ui.Hex("#58585c")
	}
	return ui.Hex("#c4c4c8")
}

func monoInk(dark bool) ui.Color {
	if dark {
		return ui.Hex("#d1d1d6")
	}
	return ui.Hex("#3a3a3c")
}

type tabBody struct {
	cx, w, h float32
	color    ui.Color
	title    string
	textW    float32
}

type tabMorphState struct {
	active           bool
	ids              []uint64
	from, to         []tabSlotGeom
	toIdx            int
	fromID, toID     uint64
	fromBody, toBody tabBody
	start            time.Time
	duration         time.Duration
}

func (m tabMorphState) progress(now time.Time) float32 {
	if !m.active || m.duration <= 0 {
		return 1
	}
	return clampF(float32(now.Sub(m.start))/float32(m.duration), 0, 1)
}

func (m tabMorphState) bodyAt(now time.Time) tabBody {
	t := m.progress(now)
	e := easeLiquid(t)
	b := m.fromBody
	b.cx += (m.toBody.cx - b.cx) * e
	b.w = max(pillHeight, b.w+(m.toBody.w-b.w)*e)
	b.h += (m.toBody.h - b.h) * e
	b.color = b.color.Mix(m.toBody.color, t)
	if t >= 0.5 {
		b.title, b.textW = m.toBody.title, m.toBody.textW
	}
	return b
}

func (a *app) cancelMorphFor(id uint64) {
	if a.morph.active && slices.Contains(a.morph.ids, id) {
		a.morph = tabMorphState{}
	}
}

func (a *app) animClock(p *ui.Painter) time.Time {
	if a.animNow != nil {
		return a.animNow()
	}
	return p.Now()
}

func (a *app) uiNow(c *ui.Context) time.Time {
	if a.animNow != nil {
		return a.animNow()
	}
	return c.Now()
}

func sameSlots(a, b []tabSlotGeom) bool { return slices.Equal(a, b) }

func (a *app) syncTabMorph(now time.Time, reduceMotion bool, slots []tabSlotGeom, ids []uint64, body tabBody) {
	activeID := a.tabs[a.active].ID
	defer func() {
		a.lastSlots = slices.Clone(slots)
		a.lastIDs = slices.Clone(ids)
		a.builtActiveID = activeID
		a.lastBody = body
	}()
	if reduceMotion || !slices.Equal(a.lastIDs, ids) || a.builtActiveID == 0 {
		a.morph = tabMorphState{}
		return
	}
	if activeID == a.builtActiveID {
		if !sameSlots(a.lastSlots, slots) || body != a.lastBody || a.morph.progress(now) >= 1 {
			a.morph = tabMorphState{}
		}
		return
	}
	from, fromBody := a.lastSlots, a.lastBody
	if a.morph.active {
		from = interpSlots(a.morph.from, a.morph.to, easeLiquid(a.morph.progress(now)))
		fromBody = a.morph.bodyAt(now)
	}
	a.morph = tabMorphState{
		active: true, ids: slices.Clone(ids), from: slices.Clone(from), to: slices.Clone(slots),
		toIdx: a.active, fromID: a.builtActiveID, toID: activeID,
		fromBody: fromBody, toBody: body, start: now, duration: a.tabAnimDuration(),
	}
}

func capsulePath(x, y, w, h float32) ui.Path {
	var p ui.Path
	r := h / 2
	const k = 0.5522847
	kk := r * k
	p.MoveTo(x+r, y)
	p.LineTo(x+w-r, y)
	p.CubeTo(x+w-r+kk, y, x+w, y+r-kk, x+w, y+r)
	p.LineTo(x+w, y+h-r)
	p.CubeTo(x+w, y+h-r+kk, x+w-r+kk, y+h, x+w-r, y+h)
	p.LineTo(x+r, y+h)
	p.CubeTo(x+r-kk, y+h, x, y+h-r+kk, x, y+h-r)
	p.LineTo(x, y+r)
	p.CubeTo(x, y+r-kk, x+r-kk, y, x+r, y)
	p.Close()
	return p
}

// The bridge meets the circular caps tangentially; its waist narrows with distance.
func liquidBridge(body tabBody, dotCX, cy float32) (ui.Path, bool) {
	gap := edgeGap(body.cx, body.w/2, dotCX, dotDiameter/2)
	var p ui.Path
	if gap < -dotDiameter/2 || gap >= bridgeReach {
		return p, false
	}
	dir := float32(1)
	if dotCX < body.cx {
		dir = -1
	}
	pr, dr := body.h/2, dotDiameter/2
	capCX := body.cx + dir*(body.w/2-pr)
	strength := clampF(1-max(0, gap)/bridgeReach, 0, 1)
	angle := float64(strength * 0.9)
	co, si := float32(math.Cos(angle)), float32(math.Sin(angle))
	x0, x1 := capCX+dir*pr*co, dotCX-dir*dr*co
	y0, y1 := pr*si, dr*si
	span := float32(math.Abs(float64(x1 - x0)))
	waist := min(y0, y1) * strength * 0.4
	midX := (x0 + x1) / 2
	control := min(span*0.25, (min(y0, y1)-waist)/co*0.8)
	p.MoveTo(x0, cy-y0)
	p.CubeTo(x0+dir*control*si, cy-y0+control*co, midX-dir*span*0.15, cy-waist, midX, cy-waist)
	p.CubeTo(midX+dir*span*0.15, cy-waist, x1-dir*control*si, cy-y1+control*co, x1, cy-y1)
	p.LineTo(x1, cy+y1)
	p.CubeTo(x1-dir*control*si, cy+y1-control*co, midX+dir*span*0.15, cy+waist, midX, cy+waist)
	p.CubeTo(midX-dir*span*0.15, cy+waist, x0+dir*control*si, cy+y0-control*co, x0, cy+y0)
	p.Close()
	return p, true
}

func (a *app) morphDotInk(id uint64, dark bool) ui.Color {
	for i := range a.tabs {
		if a.tabs[i].ID == id {
			return a.tabInk(i, dark)
		}
	}
	return emptyDotInk(dark)
}

func paintSolidDot(p *ui.Painter, cx, cy float32, ink ui.Color) {
	var path ui.Path
	path.Circle(cx, cy, dotDiameter/2)
	p.FillPath(&path, ink)
}

func paintBody(p *ui.Painter, body tabBody, cy float32) {
	path := capsulePath(body.cx-body.w/2, cy-body.h/2, body.w, body.h)
	p.FillPath(&path, body.color)
}

func paintBodyTitle(p *ui.Painter, body tabBody, cy float32, title string, textW, alpha float32) {
	if title == "" || alpha <= 0 {
		return
	}
	r := ui.Rect{X: body.cx - body.w/2 + pillPadX, Y: cy - body.h/2, W: max(0, body.w-2*pillPadX), H: body.h}
	p.Clip(r, 0, func() {
		p.Text(body.cx-textW/2, cy-pillTitleSize/2-2, title, pillTitleSize, pillTextColor(body.color).Alpha(alpha))
	})
}

func (a *app) paintTabStrip(p *ui.Painter, r ui.Rect, dark bool) {
	cy := r.Y + r.H/2
	slots, ids, body := a.lastSlots, a.lastIDs, a.lastBody
	m := a.morph
	if m.active {
		slots, ids = interpSlots(m.from, m.to, easeLiquid(m.progress(a.animClock(p)))), m.ids
		body = m.bodyAt(a.animClock(p))
	}
	body.cx += r.X
	if m.active {
		for _, s := range slots {
			if bridge, ok := liquidBridge(body, r.X+s.cx, cy); ok {
				p.FillPath(&bridge, body.color)
			}
		}
	}
	for i, s := range slots {
		if !m.active && ids[i] == a.builtActiveID {
			continue
		}
		paintSolidDot(p, r.X+s.cx, cy, a.morphDotInk(ids[i], dark))
	}
	paintBody(p, body, cy)
	if a.renaming != 0 {
		return
	}
	if m.active {
		fromA, toA := titleAlphas(m.progress(a.animClock(p)))
		paintBodyTitle(p, body, cy, m.fromBody.title, m.fromBody.textW, fromA)
		paintBodyTitle(p, body, cy, m.toBody.title, m.toBody.textW, toA)
	} else {
		paintBodyTitle(p, body, cy, body.title, body.textW, 1)
	}
}

func fitTabTitle(s string, limit float32, measure func(string) float32) (string, float32) {
	if limit <= 0 || s == "" {
		return "", 0
	}
	if w := measure(s); w <= limit {
		return s, w
	}
	if measure("…") > limit {
		return "", 0
	}
	runes := []rune(s)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if measure(string(runes[:mid])+"…") <= limit {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	s = string(runes[:lo]) + "…"
	return s, measure(s)
}

func measuredTitleWidth(s string, limit float32, measure func(string) float32) float32 {
	if measure == nil {
		return min(float32(len([]rune(s)))*pillTitleSize, limit)
	}
	return min(measure(s), limit)
}
