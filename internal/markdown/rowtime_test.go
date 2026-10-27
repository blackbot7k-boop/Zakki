package markdown

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
		{"时刻加提醒", "买牛奶 @2026-10-25 10:00 ~10m", "买牛奶 ", &TimeSuffix{base, 10 * time.Minute, true}},
		{"仅时刻", "买牛奶 @2026-10-25 10:00", "买牛奶 ", &TimeSuffix{base, 0, false}},
		{"准点档位", "x @2026-10-25 10:00 ~0m", "x ", &TimeSuffix{base, 0, true}},
		{"五分钟档位", "x @2026-10-25 10:00 ~5m", "x ", &TimeSuffix{base, 5 * time.Minute, true}},
		{"三十分钟档位", "x @2026-10-25 10:00 ~30m", "x ", &TimeSuffix{base, 30 * time.Minute, true}},
		{"小时档位", "x @2026-10-25 10:00 ~1h", "x ", &TimeSuffix{base, time.Hour, true}},
		{"一天档位", "x @2026-10-25 10:00 ~1d", "x ", &TimeSuffix{base, 24 * time.Hour, true}},
		{"CRLF 行尾", "x @2026-10-25 10:00 ~5m\r", "x ", &TimeSuffix{base, 5 * time.Minute, true}},
		{"令牌后空白", "x @2026-10-25 10:00\t ", "x ", &TimeSuffix{base, 0, false}},
		{"整行就是令牌", "@2026-10-25 10:00", "", &TimeSuffix{base, 0, false}},
		{"待办行照常识别", "- [ ] 交报告 @2026-10-25 10:00 ~1h", "- [ ] 交报告 ", &TimeSuffix{base, time.Hour, true}},
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
				if !got.Time.Equal(tc.want.Time) || got.Alert != tc.want.Alert || got.HasAlert != tc.want.HasAlert {
					t.Errorf("SplitTimeSuffix(%q) = %+v，要 %+v", tc.in, *got, *tc.want)
				}
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
		{"规范写回", "买牛奶  ", &TimeSuffix{base, 10 * time.Minute, true}, "买牛奶 @2026-10-25 10:00 ~10m"},
		{"仅时刻写回", "买牛奶 ", &TimeSuffix{base, 0, false}, "买牛奶 @2026-10-25 10:00"},
		{"准点写回", "x", &TimeSuffix{base, 0, true}, "x @2026-10-25 10:00 ~0m"},
		{"清除时间", "买牛奶 ", nil, "买牛奶"},
		{"正文带前缀原样保留", "- [ ] 交报告", &TimeSuffix{base, time.Hour, true}, "- [ ] 交报告 @2026-10-25 10:00 ~1h"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := JoinTimeSuffix(tc.in, tc.lt); got != tc.want {
				t.Errorf("JoinTimeSuffix(%q, %+v) = %q，要 %q", tc.in, tc.lt, got, tc.want)
			}
		})
	}
}

// TestJoinSplitRoundTrip 验证写回后可原样解析，且再写回结果稳定（幂等）。
func TestJoinSplitRoundTrip(t *testing.T) {
	base := time.Date(2026, 10, 25, 10, 0, 0, 0, time.Local)
	lts := []*TimeSuffix{
		{base, 0, false},
		{base, 0, true},
		{base, 5 * time.Minute, true},
		{base, 10 * time.Minute, true},
		{base, 30 * time.Minute, true},
		{base, time.Hour, true},
		{base, 24 * time.Hour, true},
	}
	for _, lt := range lts {
		once := JoinTimeSuffix("买牛奶", lt)
		text, got := SplitTimeSuffix(once)
		if got == nil {
			t.Fatalf("SplitTimeSuffix(%q) = nil，要识别出令牌", once)
		}
		if text != "买牛奶 " || !got.Time.Equal(lt.Time) || got.Alert != lt.Alert || got.HasAlert != lt.HasAlert {
			t.Fatalf("round trip 不一致: %q -> %+v", once, *got)
		}
		if twice := JoinTimeSuffix(text, got); twice != once {
			t.Errorf("Join 不幂等: %q -> %q", once, twice)
		}
	}
}
