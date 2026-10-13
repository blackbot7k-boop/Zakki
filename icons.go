package main

// 内嵌的 Lucide 图标（ISC 许可），统一 24×24 viewBox、2px 圆头描边。
// mygo 的 ui.Icon 会把它们画成文字色，尺寸用 Size 统一。

import "github.com/egoist/mygo/ui"

func lucide(inner string) *ui.SVG {
	return ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
			inner + `</svg>`))
}

var (
	// iconClearDot 清空当前点（Lucide circle-x）
	iconClearDot = lucide(`<circle cx="12" cy="12" r="10"/><path d="m15 9-6 6"/><path d="m9 9 6 6"/>`)
	// iconPin 窗口置顶（Lucide pin）
	iconPin = lucide(`<path d="M12 17v5"/><path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V6h1a2 2 0 0 0 0-4H8a2 2 0 0 0 0 4h1z"/>`)
	// iconBullet 智能圆点（Lucide asterisk）
	iconBullet = lucide(`<path d="M12 6v12"/><path d="M17.196 9 6.804 15"/><path d="m6.804 9 10.392 6"/>`)
	// iconPreview markdown 预览（Lucide eye）
	iconPreview = lucide(`<path d="M2.062 12.348a1 1 0 0 1 0-.696 10.75 10.75 0 0 1 19.876 0 1 1 0 0 1 0 .696 10.75 10.75 0 0 1-19.876 0"/><circle cx="12" cy="12" r="3"/>`)
	// iconSearch 点内搜索（Lucide search）
	iconSearch = lucide(`<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>`)
)
