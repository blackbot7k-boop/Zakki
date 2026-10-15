package app

import _ "embed"

// 「墨 Z」菜单栏图标：36×36px 对应 macOS 菜单栏 18×18pt；模板按透明度着色，其他平台保留白色。
//
//go:embed assets/tray.png
var trayIcon []byte

func trayPNG() []byte {
	return trayIcon
}
