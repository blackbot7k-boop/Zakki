package core

import (
	"strings"
	"testing"
	"time"
)

func TestRuleStrictMatrix(t *testing.T) {
	for _, s := range []string{"^0d", "^1000y", "^9999999999y", "^01w", "^2w@1", "^1mo@1", "^w@", "^w@1-5x", "^w@1-5#2", "^1d<abcdefghij", "^1d<2025-13-45", "^1d< 2025-12-31", "^1d<2025-1-1", "^w@0", "^w@8", "^w@1-2-3"} {
		if _, e := ParseRule(s); e == nil {
			t.Errorf("应拒绝 %q", s)
		}
	}
	r := mustRule(t, "^w@5,1,3,1")
	if strings.TrimSpace(marshal(r)) != `{"every":1,"unit":"w","weekdays":[1,3,5]}` {
		t.Fatalf("%+v", r)
	}
}
func TestExpandInvalidAndEmpty(t *testing.T) {
	for _, unit := range []string{"d", "w", "mo", "y"} {
		for _, n := range []int{0, -1, 1000} {
			raw := marshal(Rule{Every: n, Unit: unit})
			if _, e := Expand(raw, 0, 0, 30, 1); e == nil {
				t.Errorf("%s %d", unit, n)
			}
		}
	}
	for _, r := range []Rule{{Every: 1, Unit: "w", Weekdays: []int{9}}, {Every: 2, Unit: "w", Weekdays: []int{1}}, {Every: 1, Unit: "w", Weekdays: []int{2, 1}}} {
		if _, e := Expand(marshal(r), 0, 0, 30, 1); e == nil {
			t.Errorf("%+v", r)
		}
	}
	rule, _ := ParseRule("^1d")
	for _, tc := range []struct{ days, count int }{{-1, 1}, {10, 0}, {0, 1}} {
		got, e := Expand(rule, ms("2026-01-01 09:00"), ms("2026-01-01 09:00"), tc.days, tc.count)
		if e != nil || got != "[]" {
			t.Errorf("%+v: %s %v", tc, got, e)
		}
	}
	for _, tc := range []struct{ days, count int }{{3661, 1}, {10, 1001}} {
		if _, e := Expand(rule, 0, 0, tc.days, tc.count); e == nil {
			t.Errorf("%+v", tc)
		}
	}
}
func TestDSTAndOldAnchor(t *testing.T) {
	if e := SetLocation("America/New_York"); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = SetLocation("") })
	for _, rule := range []string{"^1d", "^1w"} {
		r := mustRule(t, rule)
		anchor := time.Date(2026, 3, 7, 2, 30, 0, 0, location()).UnixMilli()
		now := time.Date(2026, 3, 8, 10, 0, 0, 0, location()).UnixMilli()
		raw, e := Expand(marshal(r), anchor, now, 12, 20)
		if e != nil {
			t.Fatal(e)
		}
		var got []int64
		_ = unmarshal(raw, &got)
		for _, v := range got {
			if time.UnixMilli(v).In(location()).Format("15:04") != "02:30" {
				t.Fatalf("%s: %v", rule, time.UnixMilli(v).In(location()))
			}
		}
	}
	r := mustRule(t, "^w@1-7")
	raw, e := Expand(marshal(r), time.Date(1, 1, 1, 9, 0, 0, 0, location()).UnixMilli(), time.Date(2026, 1, 1, 8, 0, 0, 0, location()).UnixMilli(), 7, 10)
	if e != nil {
		t.Fatal(e)
	}
	var got []int64
	_ = unmarshal(raw, &got)
	if len(got) != 7 {
		t.Fatalf("%v", got)
	}
}
func TestOccurrenceAdvance(t *testing.T) {
	r := mustRule(t, "^1y")
	anchor := ms("2024-02-29 09:00")
	now := ms("2025-03-01 09:00")
	current, e := Occurrence(marshal(r), anchor, now)
	if e != nil || !strings.Contains(current, `"dueMs":`) {
		t.Fatalf("%s %v", current, e)
	}
	next, e := Advance(marshal(r), anchor, now)
	want := time.Date(2028, 2, 29, 9, 0, 0, 0, time.Local).UnixMilli()
	if e != nil || !strings.Contains(next, marshal(want)) {
		t.Fatalf("%s want %d %v", next, want, e)
	}
	end := mustRule(t, "^1d<2024-02-29")
	last, e := Occurrence(marshal(end), anchor, ms("2035-02-01 09:00"))
	if e != nil || !strings.Contains(last, marshal(anchor)) {
		t.Fatalf("有限序列须保留末次: %s %v", last, e)
	}
	next, e = Advance(marshal(end), anchor, anchor)
	if e != nil || !strings.Contains(next, `"ended":true`) {
		t.Fatalf("%s %v", next, e)
	}
	sparse := mustRule(t, "^999y")
	first := time.Date(1, 1, 1, 9, 0, 0, 0, location()).UnixMilli()
	current, e = Occurrence(marshal(sparse), first, time.Date(2026, 1, 1, 10, 0, 0, 0, location()).UnixMilli())
	wantSparse := time.Date(1999, 1, 1, 9, 0, 0, 0, location()).UnixMilli()
	if e != nil || !strings.Contains(current, marshal(wantSparse)) {
		t.Fatalf("稀疏旧锚点: %s %v", current, e)
	}
	next, e = Advance(marshal(sparse), first, time.Date(2026, 1, 1, 10, 0, 0, 0, location()).UnixMilli())
	wantSparse = time.Date(2998, 1, 1, 9, 0, 0, 0, location()).UnixMilli()
	if e != nil || !strings.Contains(next, marshal(wantSparse)) {
		t.Fatalf("超出展开地平线的推进: %s %v", next, e)
	}
}
