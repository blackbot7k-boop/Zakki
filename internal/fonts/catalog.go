package fonts

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Resource 固定远程资源的完整性元数据；家族名不参与缓存路径。
type Resource struct {
	ID, Family, URL, SHA256 string
	Bytes                   int64
	License                 string
}

// Catalog 返回独立副本，调用方不能改变生产下载白名单。
func Catalog() []Resource {
	return []Resource{
		{ID: "wenkai", Family: "LXGW WenKai Lite", URL: "https://raw.githubusercontent.com/lxgw/LxgwWenkai-Lite/251ab63642d1c1c5db80497d78911b7aae6917d7/fonts/TTF/LXGWWenKaiLite-Regular.ttf", SHA256: "f799a538a3355877e597a0ae04050e897932c9ae7689eb9227c2953c5dd549ea", Bytes: 13949472, License: "OFL-1.1"},
		{ID: "jetbrains", Family: "JetBrains Mono", URL: "https://raw.githubusercontent.com/JetBrains/JetBrainsMono/19371302b95d218af43299bce79ddbddd0bc364d/fonts/ttf/JetBrainsMono-Regular.ttf", SHA256: "e6fd0d7e91550b3ed2b735d4312474362c4716edc4fc0577a0f61ed782d5aed1", Bytes: 270224, License: "OFL-1.1"},
		{ID: "fira", Family: "Fira Code", URL: "https://raw.githubusercontent.com/tonsky/FiraCode/8da49d55f8b5978c5f888dd85452b79aad16cca2/distr/ttf/FiraCode-Regular.ttf", SHA256: "28c3ae21a853f1d74673384c7a0d620abb0e877b8c6cd8b64173a95512476824", Bytes: 299152, License: "OFL-1.1"},
	}
}

// Find 只按固定资源 ID 查找，不解析家族名或本地字体配置。
func Find(id string) (Resource, bool) {
	for _, r := range Catalog() {
		if r.ID == id {
			return r, true
		}
	}
	return Resource{}, false
}

// LocalKey 保留实际家族名，不接受字体栈或隐式修剪。
func LocalKey(family string) string {
	if family == "" || len(family) > 200 || !utf8.ValidString(family) || strings.TrimSpace(family) != family {
		return ""
	}
	for _, c := range family {
		if c == ',' || unicode.IsControl(c) {
			return ""
		}
	}
	return "local:" + family
}

// LocalFamily 仅还原通过同一安全校验的 local: 配置，不接收裸家族名。
func LocalFamily(key string) string {
	family, ok := strings.CutPrefix(key, "local:")
	if !ok || LocalKey(family) == "" {
		return ""
	}
	return family
}
