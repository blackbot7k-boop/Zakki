package app

import _ "embed"

// 「墨 Z」托盘图标：36×36px 对应 macOS 菜单栏 18×18pt。macOS 以模板按透明度
// 着色，固定白墨；其他平台系统不参与着色，按桌面明暗选墨色（深色任务栏用
// 白墨，浅色任务栏用黑墨），否则图标会隐入同色背景。
//
//go:embed assets/tray.png
var trayIconWhite []byte

//go:embed assets/tray-black.png
var trayIconBlack []byte

// trayPNG 返回指定墨色的托盘图标；whiteInk 为真时是白描边版。
func trayPNG(whiteInk bool) []byte {
	if whiteInk {
		return trayIconWhite
	}
	return trayIconBlack
}
