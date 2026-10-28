package core

import "testing"

func TestEmptyRuleEndRejected(t *testing.T) {
	for _, rule := range []string{"^1d<", "^1w<", "^w@1-5<"} {
		if _, err := ParseRule(rule); err == nil {
			t.Errorf("应拒绝 %q", rule)
		}
		if _, ok := FindTimeToken("@2026-10-27 09:00 " + rule); ok {
			t.Errorf("非法规则不可降级: %s", rule)
		}
	}
}

func TestAlertTierRequiresHorizontalBoundary(t *testing.T) {
	for _, prefix := range []string{"@2026-10-27 09:00 ~10m", "@2026-10-27 09:00 ^1d ~10m"} {
		for _, suffix := range []string{",", "。", "中", "\n", "\r", "\u00a0", "x", "1", "~", "^"} {
			if _, ok := FindTimeToken(prefix + suffix); ok {
				t.Errorf("不应识别 %q", prefix+suffix)
			}
		}
		for _, suffix := range []string{"", " 正文", "\t正文"} {
			token, ok := FindTimeToken(prefix + suffix)
			if !ok || !token.Value.HasAlert || token.End != len(prefix) {
				t.Errorf("应识别 %q: %+v %v", prefix+suffix, token, ok)
			}
		}
	}
}
