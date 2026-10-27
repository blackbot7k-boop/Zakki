package markdown

import (
	"strings"
	"testing"
	"time"
)

// 委托共享内核后的桌面集成抽查：保护区（行内代码/链接/转义）与 ^ 段经
// markdown 包装层的往返。语义全量用例在 zakki/core。

func TestFindTimeTokenProtectedSpans(t *testing.T) {
	const source = "@2026-11-01 10:00 ~1h"
	for _, text := range []string{"`" + source + "`", "[示例](" + source + ")", "\\" + source, "bob@example.com"} {
		t.Run(text, func(t *testing.T) {
			if token, ok := FindTimeToken(text); ok {
				t.Fatalf("非法或受保护内容不应识别: %+v", token)
			}
		})
	}
}

func TestReplaceTimeTokenPreservesRecur(t *testing.T) {
	// 移动端写入的 ^ 重复段，桌面原位更新时刻时必须原样保留
	raw := "任务 @2026-11-01 10:00 ^w@1-5 备注"
	due := time.Date(2026, 11, 2, 11, 30, 0, 0, time.Local)
	got := ReplaceTimeToken(raw, &TimeSuffix{Time: due, Recur: "^w@1-5"})
	if got != "任务 @2026-11-02 11:30 ^w@1-5 备注" {
		t.Fatalf("更新时刻应保留 ^ 段: %q", got)
	}
	if token, ok := FindTimeToken(raw); !ok || !strings.HasSuffix(raw[token.Start:token.End], "^w@1-5") {
		t.Fatalf("桌面应识别移动端写入的 ^ 段: %+v", token)
	}
}
