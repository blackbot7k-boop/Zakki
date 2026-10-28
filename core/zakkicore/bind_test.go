package zakkicore

import (
	"strings"
	"testing"
	"time"
)

func TestRuleSyntaxVersion(t *testing.T) {
	version, err := RuleSyntaxVersion()
	if err != nil || version != 1 {
		t.Fatalf("版本 = %d，错误 = %v", version, err)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	line := "任务 @2026-10-26 09:00 ~10m ^w@1-5"
	raw, e := SplitTimeSuffix(line)
	if e != nil || !strings.Contains(raw, `"alertMs":600000`) {
		t.Fatalf("%s %v", raw, e)
	}
	joined, e := JoinTimeSuffix("任务", `{"due":"2026-10-26 09:00","dueMs":1,"alertMs":600000,"hasAlert":true,"recur":"^w@1-5"}`)
	if e != nil || joined != line {
		t.Fatalf("%s %v", joined, e)
	}
	v, e := FindTimeToken(line, "null")
	if e != nil || !strings.Contains(v, `"found":true`) {
		t.Fatalf("%s %v", v, e)
	}
	replaced, e := ReplaceTimeToken(line, "null", "null")
	if e != nil || replaced != "任务" {
		t.Fatalf("%q %v", replaced, e)
	}
}
func TestBoundaries(t *testing.T) {
	line := "😀中 `@2026-10-26 09:00` @2026-10-27 09:00"
	v, e := FindTimeToken(line, `[[4,23]]`)
	if e != nil || !strings.Contains(v, `"start":24`) {
		t.Fatalf("%s %v", v, e)
	}
	for _, v := range []string{`{"due":"2026-10-26 09:00","alertMs":7000,"hasAlert":true}`, `{"due":"2026-10-26 09:00","alertMs":10}`, `{"due":"2026-10-26 09:00","recur":"^hello"}`} {
		if _, e := JoinTimeSuffix("x", v); e == nil {
			t.Fatalf("应拒绝 %s", v)
		}
	}
	if _, e := FindTimeToken(line, `[[1,2]]`); e == nil || !strings.HasPrefix(e.Error(), "invalid_offset:") {
		t.Fatalf("%v", e)
	}
}
func TestErrorCodesAndPanic(t *testing.T) {
	for _, tc := range []struct {
		code string
		call func() (string, error)
	}{
		{"invalid_json", func() (string, error) { return FindTimeToken("x", "[") }},
		{"invalid_due", func() (string, error) { return JoinTimeSuffix("x", `{"due":"2026-02-30 09:00"}`) }},
		{"invalid_alert", func() (string, error) {
			return JoinTimeSuffix("x", `{"due":"2026-10-26 09:00","hasAlert":true,"alertMs":7000}`)
		}},
		{"invalid_recur", func() (string, error) { return JoinTimeSuffix("x", `{"due":"2026-10-26 09:00","recur":"^hello"}`) }},
		{"invalid_rule", func() (string, error) { return ParseRule("^0d") }},
		{"invalid_args", func() (string, error) { return Expand(`{"every":1,"unit":"d"}`, 0, 0, 3661, 1) }},
		{"invalid_offset", func() (string, error) { return FindTimeToken("😀", `[[1,2]]`) }},
	} {
		_, e := tc.call()
		if e == nil || !strings.HasPrefix(e.Error(), tc.code+":") {
			t.Errorf("%s: %v", tc.code, e)
		}
	}
	f := func() (out string, err error) { defer guard(&out, &err); panic("trap") }
	if _, e := f(); e == nil || !strings.HasPrefix(e.Error(), "internal:") {
		t.Fatalf("panic: %v", e)
	}
}

func TestExpandBoundary(t *testing.T) {
	r, e := ParseRule("^1d")
	if e != nil {
		t.Fatal(e)
	}
	now := time.Date(2026, 1, 1, 9, 0, 0, 0, time.Local).UnixMilli()
	v, e := Expand(r, now, now, 30, 0)
	if e != nil || v != "[]" {
		t.Fatalf("%s %v", v, e)
	}
}
