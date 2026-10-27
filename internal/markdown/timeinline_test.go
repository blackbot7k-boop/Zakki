package markdown

import (
	"strings"
	"testing"
	"time"
)

func TestFindTimeToken(t *testing.T) {
	const source = "@2026-11-01 10:00 ~1h"
	for _, text := range []string{source, "交报告 " + source, "交报告 " + source + " 记得附表", source + " 和张三确认合同", "甲 " + source + "乙", "甲 " + source + "after", "甲\n" + source + " 后记"} {
		t.Run(text, func(t *testing.T) {
			token, ok := FindTimeToken(text)
			if !ok || text[token.Start:token.End] != source || token.Value.Alert != time.Hour || !token.Value.HasAlert {
				t.Fatalf("令牌前后有文字也应识别: %+v ok=%v", token, ok)
			}
			if token.Value.Time.Format(timeLayout) != "2026-11-01 10:00" {
				t.Fatalf("时刻错误: %+v", token.Value)
			}
		})
	}
	for _, text := range []string{"x @2026-02-30 10:00 y", "x @2026-11-01 25:00 y", "x @2026-11-01 9:00 y", "x @2026-11-01 10:000 y", "x @2026-11-01 10:00 ~7m y", "x @2026-11-01 10:00  ~1h y", "x @2026-11-01 10:00~1h y", "`" + source + "`", "[示例](" + source + ")", "\\" + source, "bob@example.com"} {
		t.Run(text, func(t *testing.T) {
			if token, ok := FindTimeToken(text); ok {
				t.Fatalf("非法或受保护内容不应识别: %+v", token)
			}
		})
	}
}

func TestReplaceTimeTokenKeepsSurroundingText(t *testing.T) {
	const raw = "交报告 @2026-11-01 10:00 ~1h 附上**预算表**"
	due := time.Date(2026, 11, 2, 11, 30, 0, 0, time.Local)
	value := &TimeSuffix{Time: due, Alert: 10 * time.Minute, HasAlert: true}
	if got, want := ReplaceTimeToken(raw, value), "交报告 @2026-11-02 11:30 ~10m 附上**预算表**"; got != want {
		t.Fatalf("应只原位更新令牌: got %q want %q", got, want)
	}
	if got, want := ReplaceTimeToken(raw, nil), "交报告 附上**预算表**"; got != want {
		t.Fatalf("清除不能删掉后文: got %q want %q", got, want)
	}
	if got := ReplaceTimeToken("交报告 @2026-11-01 10:00 ~1h", nil); got != "交报告" {
		t.Fatalf("行尾时间清除保持兼容: %q", got)
	}
	if got := ReplaceTimeToken("交报告", value); !strings.HasPrefix(got, "交报告 @2026-11-02 11:30") {
		t.Fatalf("无时间时追加: %q", got)
	}
}
