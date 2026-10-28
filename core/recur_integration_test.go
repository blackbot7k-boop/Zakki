package core

import (
	"testing"
	"time"
)

func assertOccurrence(t *testing.T, call func(string, int64, int64) (string, error), rule string, anchor, now, want time.Time) {
	t.Helper()
	raw, err := call(rule, anchor.UnixMilli(), now.UnixMilli())
	var got occurrence
	if err != nil {
		t.Fatal(err)
	}
	if err := unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Ended || got.DueMs != want.UnixMilli() {
		t.Fatalf("got %s, want %s", raw, want)
	}
}

func TestDailyWeeklyOccurrenceAcrossMonth(t *testing.T) {
	anchor := time.Date(2026, 10, 27, 9, 0, 0, 0, location())
	for _, tc := range []struct {
		rule               string
		now, current, next string
	}{
		{"^1d", "2026-10-28 10:00", "2026-10-28 09:00", "2026-10-29 09:00"},
		{"^1d", "2026-10-30 10:00", "2026-10-30 09:00", "2026-10-31 09:00"},
		{"^1d", "2026-10-31 10:00", "2026-10-31 09:00", "2026-11-01 09:00"},
		{"^1w", "2026-10-28 10:00", "2026-10-27 09:00", "2026-11-03 09:00"},
		{"^1w", "2026-10-30 10:00", "2026-10-27 09:00", "2026-11-03 09:00"},
		{"^1w", "2026-11-04 10:00", "2026-11-03 09:00", "2026-11-10 09:00"},
	} {
		t.Run(tc.rule+tc.now, func(t *testing.T) {
			parse := func(s string) time.Time {
				v, e := time.ParseInLocation(timeLayout, s, location())
				if e != nil {
					t.Fatal(e)
				}
				return v
			}
			rule, _ := ParseRule(tc.rule)
			assertOccurrence(t, Occurrence, rule, anchor, parse(tc.now), parse(tc.current))
			assertOccurrence(t, Advance, rule, anchor, parse(tc.now), parse(tc.next))
		})
	}
}

func TestRecurrenceAnchorSeconds(t *testing.T) {
	minute := time.Date(2026, 10, 27, 9, 0, 0, 0, location())
	anchor := minute.Add(42*time.Second + 123*time.Millisecond)
	for _, syntax := range []string{"^1d", "^1w", "^1mo", "^1y", "^w@1-7"} {
		rule, _ := ParseRule(syntax)
		assertOccurrence(t, Occurrence, rule, anchor, minute.Add(-time.Hour), minute)
		assertOccurrence(t, Occurrence, rule, anchor, anchor, minute)
		next := minute.AddDate(0, 0, 1)
		switch syntax {
		case "^1w":
			next = minute.AddDate(0, 0, 7)
		case "^1mo":
			next = minute.AddDate(0, 1, 0)
		case "^1y":
			next = minute.AddDate(1, 0, 0)
		}
		assertOccurrence(t, Advance, rule, anchor, anchor, next)
		raw, err := Expand(rule, anchor.UnixMilli(), minute.Add(-time.Hour).UnixMilli(), 1, 1)
		if err != nil || raw != marshal([]int64{minute.UnixMilli()}) {
			t.Fatalf("%s: %s %v", syntax, raw, err)
		}
	}
}

func TestDSTGapNormalizationAndRecovery(t *testing.T) {
	if err := SetLocation("America/New_York"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = SetLocation("") })
	anchor := time.Date(2026, 3, 7, 2, 30, 0, 0, location())
	gap := time.Date(2026, 3, 8, 2, 30, 0, 0, location())
	restored := time.Date(2026, 3, 9, 2, 30, 0, 0, location())
	if gap.Format(timeLayout+" -0700") != "2026-03-08 01:30 -0500" {
		t.Fatalf("gap = %s", gap)
	}
	for _, syntax := range []string{"^1d", "^w@1-7"} {
		rule, _ := ParseRule(syntax)
		raw, err := Expand(rule, anchor.UnixMilli(), anchor.UnixMilli(), 3, 2)
		if err != nil || raw != marshal([]int64{gap.UnixMilli(), restored.UnixMilli()}) {
			t.Fatalf("%s: %s %v", syntax, raw, err)
		}
		assertOccurrence(t, Advance, rule, anchor, anchor, gap)
		assertOccurrence(t, Occurrence, rule, anchor, gap, gap)
		assertOccurrence(t, Advance, rule, anchor, gap, restored)
		assertOccurrence(t, Occurrence, rule, anchor, restored, restored)
	}
}
