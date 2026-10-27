package app

import "github.com/egoist/mygo/ui"

const (
	tabPillContrastTarget = 5.0
	tabPillContrastMin    = 4.5
	tabPillMaxMix         = float32(0.12)
)

type tabPillPalette struct {
	background ui.Color
	text       ui.Color
}

// tabPillColors 只派生不透明胶囊的显示色，不改写标签原色；12 DIP 的标题
// 优先争取 5:1，背景向黑白混合最多 12%，稳定态始终保留 4.5:1 底线。
func tabPillColors(base ui.Color) tabPillPalette {
	light, dark := ui.RGB(248, 248, 250), ui.RGB(36, 36, 38)
	text, toward := light, ui.RGB(0, 0, 0)
	if contrastRatio(dark, base) > contrastRatio(light, base) {
		text, toward = dark, ui.RGB(255, 255, 255)
	}
	background := tabContrastMix(base, toward, text, tabPillContrastTarget, tabPillMaxMix)
	if contrastRatio(text, background) >= tabPillContrastMin {
		return tabPillPalette{background: background, text: text}
	}
	// 任意不透明底色至少有一种纯黑白字达到 4.5:1；只在柔和字色不足时兜底。
	white, black := ui.RGB(255, 255, 255), ui.RGB(0, 0, 0)
	extreme := white
	if contrastRatio(black, background) > contrastRatio(white, background) {
		extreme = black
	}
	text = tabContrastMix(text, extreme, background, tabPillContrastMin, 1)
	return tabPillPalette{background: background, text: text}
}

// tabContrastMix 在有界混色内寻找最小改动；预算不足时保留边界色。
func tabContrastMix(base, toward, against ui.Color, target float64, limit float32) ui.Color {
	if contrastRatio(base, against) >= target {
		return base
	}
	result := base.Mix(toward, limit)
	if contrastRatio(result, against) < target {
		return result
	}
	lo, hi := float32(0), limit
	for range 12 {
		mid := (lo + hi) / 2
		candidate := base.Mix(toward, mid)
		if contrastRatio(candidate, against) >= target {
			hi, result = mid, candidate
		} else {
			lo = mid
		}
	}
	return result
}
