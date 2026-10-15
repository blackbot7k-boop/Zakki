package app

import (
	"encoding/json"
	"os"
	"testing"
)

// TestLocalesCatalogParity 英文与简体中文目录的键位必须一一对应：
// 漏译会在英文界面下静默回退中文，该测试在提交前拦住。
func TestLocalesCatalogParity(t *testing.T) {
	read := func(path string) map[string]any {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		return m
	}
	en := read("../../locales/en.json")
	zh := read("../../locales/zh-Hans.json")
	keys := func(m map[string]any) map[string]bool {
		out := map[string]bool{}
		for k := range m {
			if k == "@@locale" || (len(k) > 0 && k[0] == '@') {
				continue
			}
			out[k] = true
		}
		return out
	}
	enKeys, zhKeys := keys(en), keys(zh)
	if len(enKeys) == 0 {
		t.Fatal("英文目录为空")
	}
	for k := range enKeys {
		if !zhKeys[k] {
			t.Errorf("zh-Hans 缺少消息 %q", k)
		}
	}
	for k := range zhKeys {
		if !enKeys[k] {
			t.Errorf("zh-Hans 多出消息 %q（英文目录没有）", k)
		}
	}
	if en["@@locale"] != "en" || zh["@@locale"] != "zh-Hans" {
		t.Errorf("@@locale 标注错误: %q / %q", en["@@locale"], zh["@@locale"])
	}
}
