package core

import (
	"reflect"
	"testing"
	"time"
)

func TestExpandReviewInstanceMatrix(t *testing.T) {
	for _, tc := range []struct {
		name, rule, anchor, now string
		horizon, count          int
		want                    []string
	}{
		{"周几集", "^w@1,3,5", "2026-01-05 09:00", "2026-01-05 09:00", 7, 64, []string{"2026-01-07 09:00", "2026-01-09 09:00", "2026-01-12 09:00"}},
		{"三日间隔", "^3d", "2026-01-01 09:00", "2026-01-08 10:00", 7, 64, []string{"2026-01-10 09:00", "2026-01-13 09:00"}},
		{"隔月月末", "^2mo", "2026-01-31 09:00", "2026-01-31 09:00", 200, 64, []string{"2026-03-31 09:00", "2026-05-31 09:00", "2026-07-31 09:00"}},
		{"隔年闰日", "^2y", "2024-02-29 09:00", "2024-02-29 09:00", 3000, 64, []string{"2028-02-29 09:00", "2032-02-29 09:00"}},
		{"单个配额", "^1d", "2026-01-01 09:00", "2026-01-01 09:00", 60, 1, []string{"2026-01-02 09:00"}},
		{"地平线含等时刻", "^1d", "2026-01-01 09:00", "2026-01-01 09:00", 1, 64, []string{"2026-01-02 09:00"}},
		{"地平线排除晚一分钟", "^1d", "2026-01-01 09:00", "2026-01-01 08:59", 1, 64, []string{"2026-01-01 09:00"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rule, err := ParseRule(tc.rule)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := Expand(rule, ms(tc.anchor), ms(tc.now), tc.horizon, tc.count)
			if err != nil {
				t.Fatal(err)
			}
			var got []int64
			if err := unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			want := make([]int64, len(tc.want))
			for i, v := range tc.want {
				want[i] = ms(v)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("实例 = %v, want %v", got, want)
			}
		})
	}
}

func TestExpandInvalidReturnsPromptlyWithoutPanic(t *testing.T) {
	type result struct {
		err   error
		panic any
	}
	inputs := []string{"{", "null", `{"every":100000000,"unit":"mo"}`, `{"every":9999999999,"unit":"y"}`}
	for _, unit := range []string{"d", "w", "mo", "y"} {
		for _, every := range []int{0, -1} {
			inputs = append(inputs, marshal(Rule{Every: every, Unit: unit}))
		}
	}
	for _, raw := range inputs {
		t.Run(raw, func(t *testing.T) {
			done := make(chan result, 1)
			go func() {
				var r result
				defer func() { r.panic = recover(); done <- r }()
				_, r.err = Expand(raw, 0, 0, 60, 64)
			}()
			select {
			case r := <-done:
				if r.err == nil || r.panic != nil {
					t.Fatalf("非法规则应返回错误：err=%v, panic=%v", r.err, r.panic)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("非法规则超过 2 秒仍未返回")
			}
		})
	}
}
