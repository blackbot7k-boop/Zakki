package core

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TimeTokenInfo 是行内时间令牌簇的 JSON 解析结果，供 gomobile 边界使用
// （只跨字符串、数值与布尔；结构体字段全部可序列化）。
type TimeTokenInfo struct {
	Start    int    `json:"start"`             // 令牌簇在行内的起始字节偏移
	End      int    `json:"end"`               // 结束偏移（不含）
	Due      string `json:"due"`               // YYYY-MM-DD HH:mm
	DueMs    int64  `json:"dueMs"`             // 本地时区 Unix 毫秒
	AlertMs  int64  `json:"alertMs,omitempty"` // 提前量毫秒；无提醒为 0
	HasAlert bool   `json:"hasAlert"`
	Recur    string `json:"recur,omitempty"` // ^ 段原文，如 "^w@1-5"
}

var tokenRe = regexp.MustCompile(`@(\d{4}-\d{2}-\d{2}) (\d{2}:\d{2})`)

// ParseTimeToken 找出行内第一个时间令牌簇：@时刻（必须）+ ~提前量（可选）+
// ^规则（可选），返回 TimeTokenInfo 的 JSON。原型边界入口：不做保护区排除
// （代码/链接中的示例由渲染层自行规避），错误以非 nil error 返回。
func ParseTimeToken(line string) (string, error) {
	loc := tokenRe.FindStringSubmatchIndex(line)
	if loc == nil {
		return "", fmt.Errorf("行内无时间令牌")
	}
	start := loc[0]
	end := loc[1]
	dueStr := line[loc[2]:loc[3]] + " " + line[loc[4]:loc[5]]
	due, err := time.ParseInLocation("2006-01-02 15:04", dueStr, time.Local)
	if err != nil {
		return "", err
	}
	info := TimeTokenInfo{Start: start, End: end, Due: dueStr, DueMs: due.UnixMilli()}
	rest := line[end:]
	if strings.HasPrefix(rest, " ") {
		if m := regexp.MustCompile(`^ ~(\d+)([mhd])`).FindStringSubmatch(rest); m != nil {
			n, _ := strconv.Atoi(m[1])
			info.AlertMs = int64(n) * alertUnitMs(m[2])
			info.HasAlert = true
			end += len(m[0])
		}
	}
	rest = line[end:]
	if i := strings.Index(rest, "^"); i >= 0 && (i == 0 || rest[i-1] == ' ') {
		j := i + 1
		// ^ 段内含 @（周几集）与 <（结束日期），仅空格与 ~ 终止
		for j < len(rest) && !strings.ContainsRune(" ~", rune(rest[j])) {
			j++
		}
		rule := rest[i:j]
		if _, err := ParseRule(rule); err != nil {
			return "", err
		}
		info.Recur = rule
		end += j
	}
	info.End = end
	return marshal(info), nil
}

func alertUnitMs(u string) int64 {
	switch u {
	case "h":
		return 3600_000
	case "d":
		return 86400_000
	default:
		return 60_000
	}
}
