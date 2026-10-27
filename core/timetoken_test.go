package core

import (
	"testing"
	"time"
)

func rowTimeCases() ([]struct {
	name string
	in   string
	text string
	want *TimeSuffix
}, time.Time) {
	base := time.Date(2026, 10, 25, 10, 0, 0, 0, time.Local)
	return []struct {
		name string
		in   string
		text string
		want *TimeSuffix
	}{
		{"时刻加提醒", "买牛奶 @2026-10-25 10:00 ~10m", "买牛奶 ", &TimeSuffix{Time: base, Alert: 10 * time.Minute, HasAlert: true}},
		{"仅时刻", "买牛奶 @2026-10-25 10:00", "买牛奶 ", &TimeSuffix{Time: base}},
		{"准点档位", "x @2026-10-25 10:00 ~0m", "x ", &TimeSuffix{Time: base, HasAlert: true}},
		{"五分钟档位", "x @2026-10-25 10:00 ~5m", "x ", &TimeSuffix{Time: base, Alert: 5 * time.Minute, HasAlert: true}},
		{"三十分钟档位", "x @2026-10-25 10:00 ~30m", "x ", &TimeSuffix{Time: base, Alert: 30 * time.Minute, HasAlert: true}},
		{"小时档位", "x @2026-10-25 10:00 ~1h", "x ", &TimeSuffix{Time: base, Alert: time.Hour, HasAlert: true}},
		{"一天档位", "x @2026-10-25 10:00 ~1d", "x ", &TimeSuffix{Time: base, Alert: 24 * time.Hour, HasAlert: true}},
		{"CRLF 行尾", "x @2026-10-25 10:00 ~5m\r", "x ", &TimeSuffix{Time: base, Alert: 5 * time.Minute, HasAlert: true}},
		{"令牌后空白", "x @2026-10-25 10:00\t ", "x ", &TimeSuffix{Time: base}},
		{"整行就是令牌", "@2026-10-25 10:00", "", &TimeSuffix{Time: base}},
		{"待办行照常识别", "- [ ] 交报告 @2026-10-25 10:00 ~1h", "- [ ] 交报告 ", &TimeSuffix{Time: base, Alert: time.Hour, HasAlert: true}},
		{"无令牌", "买牛奶", "买牛奶", nil},
		{"空行", "", "", nil},
		{"行中令牌不算", "x @2026-10-25 10:00 y", "x @2026-10-25 10:00 y", nil},
		{"非法日期", "x @2026-02-30 10:00", "x @2026-02-30 10:00", nil},
		{"非法时刻", "x @2026-10-25 25:00", "x @2026-10-25 25:00", nil},
		{"非零填充月份", "x @2026-1-5 09:00", "x @2026-1-5 09:00", nil},
		{"非零填充时刻", "x @2026-10-25 9:00", "x @2026-10-25 9:00", nil},
		{"档位白名单外", "x @2026-10-25 10:00 ~7m", "x @2026-10-25 10:00 ~7m", nil},
		{"有提醒无时刻", "x ~10m", "x ~10m", nil},
		{"缺空格不算", "x @2026-10-25 10:00~10m", "x @2026-10-25 10:00~10m", nil},
		{"多空格不算", "x @2026-10-25 10:00  ~10m", "x @2026-10-25 10:00  ~10m", nil},
		{"大写单位不算", "x @2026-10-25 10:00 ~10M", "x @2026-10-25 10:00 ~10M", nil},
		{"邮箱行不受影响", "发给 bob@example.com", "发给 bob@example.com", nil},
	}, base
}

func TestSplitTimeSuffix(t *testing.T) {
	cases, _ := rowTimeCases()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text, got := SplitTimeSuffix(tc.in)
			if text != tc.text {
				t.Errorf("SplitTimeSuffix(%q) 正文 = %q，要 %q", tc.in, text, tc.text)
			}
			switch {
			case got == nil && tc.want != nil:
				t.Fatalf("SplitTimeSuffix(%q) = nil，要 %+v", tc.in, *tc.want)
			case got != nil && tc.want == nil:
				t.Fatalf("SplitTimeSuffix(%q) = %+v，要 nil", tc.in, *got)
			case got != nil && tc.want != nil:
				if !got.Time.Equal(tc.want.Time) || got.Alert != tc.want.Alert || got.HasAlert != tc.want.HasAlert || got.Recur != tc.want.Recur {
					t.Errorf("SplitTimeSuffix(%q) = %+v，要 %+v", tc.in, *got, *tc.want)
				}
			}
		})
	}
}

func TestSplitTimeSuffixRecur(t *testing.T) {
	cases := []struct {
		name string
		in   string
		text string
		want *TimeSuffix
	}{
		{"时刻加重复", "健身 @2026-10-26 09:00 ^w@1-5", "健身 ", &TimeSuffix{Time: time.Date(2026, 10, 26, 9, 0, 0, 0, time.Local), Recur: "^w@1-5"}},
		{"提醒加重复", "x @2026-10-26 09:00 ~10m ^1w", "x ", &TimeSuffix{Time: time.Date(2026, 10, 26, 9, 0, 0, 0, time.Local), Alert: 10 * time.Minute, HasAlert: true, Recur: "^1w"}},
		{"带结束日期", "x @2026-10-26 09:00 ^1mo<2026-12-31", "x ", &TimeSuffix{Time: time.Date(2026, 10, 26, 9, 0, 0, 0, time.Local), Recur: "^1mo<2026-12-31"}},
		{"非法重复整行不算（行尾严格，同提前量白名单外）", "x @2026-10-26 09:00 ^hello", "x @2026-10-26 09:00 ^hello", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text, got := SplitTimeSuffix(tc.in)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("SplitTimeSuffix(%q) = %+v，要 nil", tc.in, *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("SplitTimeSuffix(%q) = nil，要 %+v", tc.in, *tc.want)
			}
			if text != tc.text || !got.Time.Equal(tc.want.Time) || got.Recur != tc.want.Recur {
				t.Errorf("SplitTimeSuffix(%q) = (%q, %+v)，要 (%q, %+v)", tc.in, text, *got, tc.text, *tc.want)
			}
		})
	}
}

func TestJoinTimeSuffix(t *testing.T) {
	base := time.Date(2026, 10, 25, 10, 0, 0, 0, time.Local)
	cases := []struct {
		name string
		in   string
		lt   *TimeSuffix
		want string
	}{
		{"规范写回", "买牛奶  ", &TimeSuffix{Time: base, Alert: 10 * time.Minute, HasAlert: true}, "买牛奶 @2026-10-25 10:00 ~10m"},
		{"仅时刻写回", "买牛奶 ", &TimeSuffix{Time: base}, "买牛奶 @2026-10-25 10:00"},
		{"准点写回", "x", &TimeSuffix{Time: base, HasAlert: true}, "x @2026-10-25 10:00 ~0m"},
		{"清除时间", "买牛奶 ", nil, "买牛奶"},
		{"正文带前缀原样保留", "- [ ] 交报告", &TimeSuffix{Time: base, Alert: time.Hour, HasAlert: true}, "- [ ] 交报告 @2026-10-25 10:00 ~1h"},
		{"重复规则写回", "健身", &TimeSuffix{Time: base, Recur: "^w@1-5"}, "健身 @2026-10-25 10:00 ^w@1-5"},
		{"非法重复不写回", "健身", &TimeSuffix{Time: base, Recur: "^hello"}, "健身 @2026-10-25 10:00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := JoinTimeSuffix(tc.in, tc.lt); got != tc.want {
				t.Errorf("JoinTimeSuffix(%q, %+v) = %q，要 %q", tc.in, tc.lt, got, tc.want)
			}
		})
	}
}

// 行内令牌簇：^ 段识别、非法 ^ 宽松忽略、保护区排除与推进改写。
func TestFindTimeTokenRecur(t *testing.T) {
	const src = "任务 @2026-10-26 09:00 ^w@1-5"
	token, ok := FindTimeToken(src)
	if !ok || token.Value.Recur != "^w@1-5" || src[token.Start:token.End] != "@2026-10-26 09:00 ^w@1-5" {
		t.Fatalf("^ 段应纳入令牌: %+v ok=%v", token, ok)
	}
	// 非法 ^ 宽松忽略：令牌仍成立，段保持普通文本
	token, ok = FindTimeToken("任务 @2026-10-26 09:00 ^hello")
	if !ok || token.Value.Recur != "" || token.End != token.Start+timeTokenLen {
		t.Fatalf("非法 ^ 应忽略且令牌仍成立: %+v ok=%v", token, ok)
	}
	// 更新时刻时保留 ^ 段
	const raw = "任务 @2026-10-26 09:00 ^w@1-5 备注"
	due := time.Date(2026, 10, 27, 8, 30, 0, 0, time.Local)
	got := ReplaceTimeToken(raw, &TimeSuffix{Time: due, Recur: "^w@1-5"})
	if got != "任务 @2026-10-27 08:30 ^w@1-5 备注" {
		t.Fatalf("推进后应保留 ^ 段: %q", got)
	}
	// 保护区内的 @ 不识别
	if _, ok := FindTimeTokenEx("`@2026-10-26 09:00`", [][2]int{{0, 18}}); ok {
		t.Fatal("保护区内的 @ 不应识别")
	}
}

func TestJoinSplitRoundTrip(t *testing.T) {
	base := time.Date(2026, 10, 25, 10, 0, 0, 0, time.Local)
	lts := []*TimeSuffix{
		{Time: base},
		{Time: base, HasAlert: true},
		{Time: base, Alert: 5 * time.Minute, HasAlert: true},
		{Time: base, Alert: 10 * time.Minute, HasAlert: true},
		{Time: base, Alert: 30 * time.Minute, HasAlert: true},
		{Time: base, Alert: time.Hour, HasAlert: true},
		{Time: base, Alert: 24 * time.Hour, HasAlert: true},
		{Time: base, Recur: "^1d"},
		{Time: base, Alert: 10 * time.Minute, HasAlert: true, Recur: "^w@1-5"},
		{Time: base, Recur: "^2w<2026-12-31"},
	}
	for _, lt := range lts {
		once := JoinTimeSuffix("买牛奶", lt)
		text, got := SplitTimeSuffix(once)
		if got == nil {
			t.Fatalf("SplitTimeSuffix(%q) = nil，要识别出令牌", once)
		}
		if text != "买牛奶 " || !got.Time.Equal(lt.Time) || got.Alert != lt.Alert || got.HasAlert != lt.HasAlert || got.Recur != lt.Recur {
			t.Fatalf("round trip 不一致: %q -> %+v", once, *got)
		}
		if twice := JoinTimeSuffix(text, got); twice != once {
			t.Errorf("Join 不幂等: %q -> %q", once, twice)
		}
	}
}
