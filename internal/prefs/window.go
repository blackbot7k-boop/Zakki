package prefs

import (
	"fmt"
	"math"
)

// 默认窗口尺寸按「每行字数 × 行数」记录，开窗时按当时生效的字号、行高与排版
// 内边距换算像素，改字号后新窗口仍放得下同样多的字。
const (
	MinWindowColumns = 10
	MaxWindowColumns = 160
	MinWindowRows    = 4
	MaxWindowRows    = 120
)

// WindowMetrics 列数 × 行数与窗口像素互换所需的排版量（DIP）。
type WindowMetrics struct {
	BodySize, LineHeight float32 // 正文字号与行高倍数
	PadX, PadY           float32 // 正文区左右 / 上下内边距
	Chrome               float32 // 顶栏与状态栏的总高度
}

// WindowPixels 列数 × 行数换算为窗口像素宽高（向上取整）；汉字按 1em 宽计。
func WindowPixels(cols, rows int, m WindowMetrics) (int, int) {
	w := float32(cols)*m.BodySize + 2*m.PadX
	h := float32(rows)*m.BodySize*m.LineHeight + 2*m.PadY + m.Chrome
	return int(math.Ceil(float64(w))), int(math.Ceil(float64(h)))
}

// WindowGrid 窗口像素宽高换算为列数 × 行数：四舍五入到整字整行并限定在合法范围。
func WindowGrid(w, h int, m WindowMetrics) (int, int) {
	cols := math.Round(float64((float32(w) - 2*m.PadX) / m.BodySize))
	rows := math.Round(float64((float32(h) - 2*m.PadY - m.Chrome) / (m.BodySize * m.LineHeight)))
	return min(max(int(cols), MinWindowColumns), MaxWindowColumns), min(max(int(rows), MinWindowRows), MaxWindowRows)
}

// legacyWindowMetrics 迁移旧像素尺寸用的排版量：偏好里没有排版表，按默认
// Sonnet 排版（16 / 1.6、内边距 24）与 macOS 顶栏 38 + 状态栏 36 换算，有字号 /
// 行高覆盖时用覆盖值；误差在一个字、一行以内。
func legacyWindowMetrics(a Appearance) WindowMetrics {
	m := WindowMetrics{BodySize: 16, LineHeight: 1.6, PadX: 24, PadY: 24, Chrome: 74}
	if a.BodySize > 0 {
		m.BodySize = a.BodySize
	}
	if a.LineHeight > 0 {
		m.LineHeight = a.LineHeight
	}
	return m
}

// stripLegacyStorageKeys 摘除版本 12 移除的旧备份与防抖键：backup 整节与
// advanced.autosaveDelayMs。旧语义不迁移、不映射，只从待解码映射里删除，
// 写回即为版本 12；版本 12 及以上的文件保持原样，交给严格解码拒绝。
// 返回是否摘除了键。
func stripLegacyStorageKeys(value any) bool {
	m, ok := value.(map[string]any)
	if !ok {
		return false
	}
	if ver, _ := m["version"].(float64); ver >= 12 {
		return false
	}
	stripped := false
	if _, has := m["backup"]; has {
		delete(m, "backup")
		stripped = true
	}
	if adv, ok := m["advanced"].(map[string]any); ok {
		if _, has := adv["autosaveDelayMs"]; has {
			delete(adv, "autosaveDelayMs")
			stripped = true
		}
	}
	return stripped
}

// stripLegacyWindowKeys 从已解码的偏好映射里摘除版本 11 移除的像素尺寸键，
// 返回其值（缺键为 0）与是否摘除了键。旧键仍按原规则校验（整数、宽 300–4096、
// 高 340–4096），非法旧值照旧拒绝而不是静默换成默认。版本 11 及以上的文件
// 保持原样交给严格解码拒绝。
func stripLegacyWindowKeys(value any) (int, int, bool, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return 0, 0, false, nil
	}
	if ver, _ := m["version"].(float64); ver >= 11 {
		return 0, 0, false, nil
	}
	adv, ok := m["advanced"].(map[string]any)
	if !ok {
		return 0, 0, false, nil
	}
	read := func(key string, lo int) (int, bool, error) {
		raw, has := adv[key]
		if !has {
			return 0, false, nil
		}
		delete(adv, key)
		f, ok := raw.(float64)
		if !ok || f != math.Trunc(f) || f < float64(lo) || f > 4096 {
			return 0, true, fmt.Errorf("advanced.%s 的值 %v 无效，取值范围 %d–4096", key, raw, lo)
		}
		return int(f), true, nil
	}
	w, hasW, err := read("windowWidth", 300)
	if err != nil {
		return 0, 0, true, err
	}
	h, hasH, err := read("windowHeight", 340)
	if err != nil {
		return 0, 0, true, err
	}
	return w, h, hasW || hasH, nil
}
