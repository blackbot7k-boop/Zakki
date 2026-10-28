package core

import (
	"testing"
	"time"
)

func mustRule(t *testing.T, s string) Rule {
	t.Helper()
	rs, err := ParseRule(s)
	if err != nil {
		t.Fatal(err)
	}
	var r Rule
	if err := unmarshal(rs, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

// expand 包装 Expand：锚点/当下用 "2006-01-02 15:04" 字面量，返回展开时刻列表。
func expand(t *testing.T, rule Rule, anchor, now string, horizonDays, max int) []time.Time {
	t.Helper()
	s, err := Expand(marshal(rule), ms(anchor), ms(now), horizonDays, max)
	if err != nil {
		t.Fatal(err)
	}
	var out []int64
	if err := unmarshal(s, &out); err != nil {
		t.Fatal(err)
	}
	times := make([]time.Time, len(out))
	for i, v := range out {
		times[i] = time.UnixMilli(v)
	}
	return times
}

func days(times []time.Time) []string {
	var out []string
	for _, t := range times {
		out = append(out, t.Format("2006-01-02"))
	}
	return out
}

func ms(s string) int64 {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
	if err != nil {
		panic(err)
	}
	return t.UnixMilli()
}

// 每个工作日：跨周末，周末不发生
func TestExpandWeekdays(t *testing.T) {
	r := mustRule(t, "^w@1-5")
	// 锚点 2025-01-06 周一 09:00；现在=周五 01-10 10:00（当日实例已过）；地平线 7 天
	got := days(expand(t, r, "2025-01-06 09:00", "2025-01-10 10:00", 7, 64))
	want := []string{"2025-01-13", "2025-01-14", "2025-01-15", "2025-01-16", "2025-01-17"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 个 = %s, want %s", i, got[i], want[i])
		}
	}
}

// 月末跳过：1/31 起每月，2 月不发生，3/31 是第一个未来实例
func TestExpandMonthlySkip(t *testing.T) {
	r := mustRule(t, "^1mo")
	got := days(expand(t, r, "2025-01-31 09:00", "2025-01-31 10:00", 125, 64))
	for _, d := range got {
		if d[:7] == "2025-02" {
			t.Fatalf("2 月不应有实例: %v", got)
		}
	}
	if len(got) != 2 || got[0] != "2025-03-31" || got[1] != "2025-05-31" {
		t.Fatalf("got %v", got)
	}
}

// 闰日跳过：2/29 起每年，平年不发生，下一个实例是 2028-02-29
func TestExpandYearlyLeapSkip(t *testing.T) {
	r := mustRule(t, "^1y")
	got := days(expand(t, r, "2024-02-29 09:00", "2024-03-01 10:00", 1500, 64))
	if len(got) == 0 || got[0] != "2028-02-29" {
		t.Fatalf("got %v", got)
	}
}

// 双周保周几 + 结束日期含当日；配额上限生效
func TestExpandBiweeklyEndCap(t *testing.T) {
	r := mustRule(t, "^2w<2025-02-01")
	// 锚点周三 2025-01-01；结束 2/1 → 只应有 1/15、1/29
	got := days(expand(t, r, "2025-01-01 09:00", "2025-01-01 09:00", 60, 64))
	if len(got) != 2 || got[0] != "2025-01-15" || got[1] != "2025-01-29" {
		t.Fatalf("got %v", got)
	}
	r2 := mustRule(t, "^1d")
	got2 := expand(t, r2, "2025-01-01 09:00", "2025-01-01 09:00", 30, 3)
	if len(got2) != 3 {
		t.Fatalf("配额上限失效: %d", len(got2))
	}
}

// 锚点在未来：锚点即首实例；错过不补发
func TestExpandFutureAnchorAndMissed(t *testing.T) {
	r := mustRule(t, "^1d")
	got := days(expand(t, r, "2025-01-10 09:00", "2025-01-01 10:00", 30, 64))
	if got[0] != "2025-01-10" {
		t.Fatalf("未来锚点应为首实例: %v", got)
	}
	got2 := days(expand(t, r, "2025-01-01 09:00", "2025-01-10 10:00", 30, 64))
	if got2[0] != "2025-01-11" {
		t.Fatalf("错过不补，应从 01-11 开始: %v", got2)
	}
}

// 行内令牌与行尾使用同一解析器。
func TestFindTimeTokenRecurRule(t *testing.T) {
	token, ok := FindTimeToken("- [ ] 交房租 @2025-01-06 09:00 ~10m ^w@1-5")
	if !ok || !token.Value.HasAlert || token.Value.Alert != 10*time.Minute || token.Value.Recur != "^w@1-5" {
		t.Fatalf("%+v %v", token, ok)
	}
	if _, ok := FindTimeToken("@2025-01-06 09:00 ^5x"); ok {
		t.Fatal("非法规则不能退化为一次性提醒")
	}
}
