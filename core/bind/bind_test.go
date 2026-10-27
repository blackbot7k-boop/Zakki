package bind

import (
	"strings"
	"testing"
	"time"
)

// JSON 边界的往返抽查：解析 → 改写 → 再解析，语义须与 core 的 Go API 一致。
func TestTokenJSONRoundTrip(t *testing.T) {
	line := "任务 @2026-10-26 09:00 ~10m ^w@1-5 备注"
	info, err := ParseTimeToken(line)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(info, `"alertMs":600000`) || !strings.Contains(info, `"recur":"^w@1-5"`) {
		t.Fatalf("ParseTimeToken 信息不全: %s", info)
	}
	// 从解析结果构造改写（完成推进：时刻改到下一次，保留提醒与重复）
	next := `{"due":"2026-11-02 09:00","alertMs":600000,"hasAlert":true,"recur":"^w@1-5"}`
	got, err := ReplaceTimeTokenJSON(line, next)
	if err != nil {
		t.Fatal(err)
	}
	if want := "任务 @2026-11-02 09:00 ~10m ^w@1-5 备注"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	// 再解析应与新时刻一致
	info2, err := ParseTimeToken(got)
	if err != nil || !strings.Contains(info2, `"due":"2026-11-02 09:00"`) {
		t.Fatalf("改写后解析不一致: %s (%v)", info2, err)
	}
}

func TestSuffixJSONNullAndClear(t *testing.T) {
	out, err := SplitTimeSuffixJSON("没有令牌")
	if err != nil {
		t.Fatal(err)
	}
	if out != `{"text":"没有令牌","suffix":null}` {
		t.Fatalf("got %s", out)
	}
	cleared, err := JoinTimeSuffixJSON("买牛奶 ", "")
	if err != nil || cleared != "买牛奶" {
		t.Fatalf("got %q (%v)", cleared, err)
	}
}

// Expand 与非重复令牌的写回：准点档位与非法重复的边界。
func TestExpandAndEdgeCases(t *testing.T) {
	rule, err := ParseRule("^1d")
	if err != nil {
		t.Fatal(err)
	}
	anchor := time.Date(2026, 1, 1, 9, 0, 0, 0, time.Local).UnixMilli()
	now := anchor
	got, err := Expand(rule, anchor, now, 30, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "[") {
		t.Fatalf("Expand 应返回数组 JSON: %s", got)
	}
	// 非法重复经 JSON 边界写回时按 core 语义宽松降级（去掉段、不报错）——
	// 与桌面 JoinTimeSuffix 的白名单哲学一致；校验错误只在 ParseRule 层报。
	got2, err := ReplaceTimeTokenJSON("x @2026-01-01 09:00", `{"due":"2026-01-02 09:00","recur":"^hello"}`)
	if err != nil || got2 != "x @2026-01-02 09:00" {
		t.Fatalf("非法 ^ 应宽松降级: %q (%v)", got2, err)
	}
	if _, err := ParseRule("^hello"); err == nil {
		t.Fatal("ParseRule 对非法规则应报错")
	}
}
