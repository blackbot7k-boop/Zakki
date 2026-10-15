// Zakki：基于 MyGo 的原生 Markdown 便签。
// 根目录只保留程序入口，UI 与装配在 internal/app，纯逻辑与持久化在 internal 各包。
package main

import "zakki/internal/app"

func main() {
	app.Run()
}
