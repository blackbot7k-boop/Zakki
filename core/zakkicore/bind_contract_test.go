package zakkicore

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode"
	"zakki/core"
)

func TestAlertTiersContract(t *testing.T) {
	raw, err := AlertTiers()
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	want := core.AlertTiers()
	if len(got) != len(want) {
		t.Fatalf("档位数 = %d，期望 %d", len(got), len(want))
	}
	for i, tier := range got {
		var suffix string
		var ms int64
		if len(tier) != 2 || tier["suffix"] == nil || tier["ms"] == nil {
			t.Fatalf("档位字段 = %s", raw)
		}
		if err := json.Unmarshal(tier["suffix"], &suffix); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(tier["ms"], &ms); err != nil {
			t.Fatal(err)
		}
		if suffix != want[i].Suffix || ms != want[i].Span.Milliseconds() {
			t.Fatalf("档位 = %s，期望 %v", raw, want)
		}
	}
}

func TestProtectedJSONStrictShape(t *testing.T) {
	line := "中😀 @2026-10-26 09:00"
	for _, tc := range []struct{ raw, code string }{
		{`[null]`, "invalid_offset"}, {`[[]]`, "invalid_offset"}, {`[[0]]`, "invalid_offset"},
		{`[[0,1,2]]`, "invalid_offset"}, {`[[null,1]]`, "invalid_offset"}, {`[[0,null]]`, "invalid_offset"},
		{`[[-1,1]]`, "invalid_offset"}, {`[[3,1]]`, "invalid_offset"}, {`[[0,23]]`, "invalid_offset"},
		{`[[1,2]]`, "invalid_offset"}, {`[[2,3]]`, "invalid_offset"},
		{`[[0,1.5]]`, "invalid_json"}, {`[[0,"1"]]`, "invalid_json"},
		{`[[0,9223372036854775808]]`, "invalid_json"}, {`{}`, "invalid_json"}, {`[`, "invalid_json"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			for _, call := range []func() (string, error){
				func() (string, error) { return FindTimeToken(line, tc.raw) },
				func() (string, error) { return ReplaceTimeToken(line, "null", tc.raw) },
			} {
				out, err := call()
				if out != "" || err == nil || !strings.HasPrefix(err.Error(), tc.code+":") {
					t.Fatalf("输出 = %q，错误 = %v，期望 %s", out, err, tc.code)
				}
			}
		})
	}
	for _, raw := range []string{"", "null", "[]", "[[0,1],[1,3],[3,4]]", "[[0,0],[21,21]]"} {
		out, err := FindTimeToken(line, raw)
		if err != nil || !strings.Contains(out, `"start":4`) || !strings.Contains(out, `"end":21`) {
			t.Fatalf("%s: %s %v", raw, out, err)
		}
	}
	out, err := FindTimeToken(line, `[[4,21]]`)
	if err != nil || !strings.Contains(out, `"found":false`) {
		t.Fatalf("保护中文 emoji 后令牌: %s %v", out, err)
	}
	out, err = ReplaceTimeToken(line, "null", `[[4,21]]`)
	if err != nil || out != line {
		t.Fatalf("保护令牌写回 = %q %v", out, err)
	}
}

func TestRuleJSONErrorClassification(t *testing.T) {
	for _, raw := range []string{`[`, `{"every":"1","unit":"d"}`, `[]`, `true`, `"rule"`} {
		for _, call := range []func() (string, error){
			func() (string, error) { return Expand(raw, 0, 0, 1, 1) },
			func() (string, error) { return Occurrence(raw, 0, 0) },
			func() (string, error) { return Advance(raw, 0, 0) },
		} {
			out, err := call()
			if out != "" || err == nil || !strings.HasPrefix(err.Error(), "invalid_json: JSON 格式非法: ") {
				t.Fatalf("%s: %q %v", raw, out, err)
			}
			var syntax *json.SyntaxError
			var mismatch *json.UnmarshalTypeError
			if !errors.As(err, &syntax) && !errors.As(err, &mismatch) {
				t.Fatalf("未保留底层 JSON 错误: %v", err)
			}
		}
	}
	for _, call := range []func() (string, error){
		func() (string, error) { return Occurrence(`{"every":0,"unit":"d"}`, 0, 0) },
		func() (string, error) { return Advance(`{"every":0,"unit":"d"}`, 0, 0) },
	} {
		_, err := call()
		if err == nil || !strings.HasPrefix(err.Error(), "invalid_rule:") {
			t.Fatalf("规则错误: %v", err)
		}
	}
}

func TestSuffixDueMsIsOutputOnly(t *testing.T) {
	const due = `{"due":"2026-10-26 09:00","dueMs":%s}`
	for _, ignored := range []string{`1`, `"忽略"`, `null`, `{}`, `[]`, `true`} {
		for _, call := range []func() (string, error){
			func() (string, error) { return JoinTimeSuffix("任务", fmt.Sprintf(due, ignored)) },
			func() (string, error) { return ReplaceTimeToken("任务", fmt.Sprintf(due, ignored), "null") },
		} {
			out, err := call()
			if err != nil || out != "任务 @2026-10-26 09:00" {
				t.Fatalf("dueMs %s: %q %v", ignored, out, err)
			}
			split, err := SplitTimeSuffix(out)
			if err != nil {
				t.Fatal(err)
			}
			var decoded struct {
				Suffix struct {
					DueMs int64 `json:"dueMs"`
				} `json:"suffix"`
			}
			if err := json.Unmarshal([]byte(split), &decoded); err != nil {
				t.Fatal(err)
			}
			want, parseErr := time.ParseInLocation("2006-01-02 15:04", "2026-10-26 09:00", core.Location())
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			if decoded.Suffix.DueMs != want.UnixMilli() {
				t.Fatalf("派生时间戳 = %d，期望 %d", decoded.Suffix.DueMs, want.UnixMilli())
			}
		}
	}
}

func TestErrorsHaveChineseExplanation(t *testing.T) {
	for _, call := range []func() (string, error){
		func() (string, error) { return JoinTimeSuffix("任务", `[`) },
		func() (string, error) { return SetLocation("不存在/时区") },
		func() (string, error) { return ParseRule("^0d") },
		func() (string, error) { return FindTimeToken("中😀", `[[2,3]]`) },
	} {
		_, err := call()
		if err == nil || !strings.ContainsFunc(err.Error(), func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
			t.Fatalf("缺少中文说明: %v", err)
		}
	}
	f := func() (out string, err error) { defer guard(&out, &err); out = "部分结果"; panic("trap") }
	out, err := f()
	if out != "" || err == nil || err.Error() != "internal: 内部处理异常" {
		t.Fatalf("panic 输出 = %q %v", out, err)
	}
	versionPanic := func() (version int, err error) {
		defer guard(&version, &err)
		version = 1
		panic("trap")
	}
	version, err := versionPanic()
	if version != 0 || err == nil || err.Error() != "internal: 内部处理异常" {
		t.Fatalf("panic 版本 = %d %v", version, err)
	}
}

func TestRecurrenceAndLocationBinding(t *testing.T) {
	previous := core.Location()
	t.Cleanup(func() {
		if _, err := SetLocation(previous.String()); err != nil {
			t.Error(err)
		}
	})
	if _, err := SetLocation(" UTC "); err != nil {
		t.Fatal(err)
	}
	rule, err := ParseRule("^1d")
	if err != nil {
		t.Fatal(err)
	}
	anchor := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC).UnixMilli()
	for _, tc := range []struct {
		call func(string, int64, int64) (string, error)
		want func(string, int64, int64) (string, error)
	}{
		{Occurrence, core.Occurrence},
		{Advance, core.Advance},
	} {
		out, err := tc.call(rule, anchor, anchor)
		want, coreErr := tc.want(rule, anchor, anchor)
		if err != nil || coreErr != nil || out != want {
			t.Fatalf("实例 = %s %v，内核结果 = %s %v", out, err, want, coreErr)
		}
	}
	if _, err := SetLocation(""); err != nil || core.Location() != time.Local {
		t.Fatalf("恢复本地时区: %v", err)
	}
}
