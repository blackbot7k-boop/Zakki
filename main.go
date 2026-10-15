// Tot MD：基于 MyGo 的 macOS Markdown 便签。
// 根目录只保留程序入口，UI 与装配在 internal/app，纯逻辑与持久化在 internal 各包。
package main

import "tot-md-mygo/internal/app"

func main() {
	app.Run()
}
