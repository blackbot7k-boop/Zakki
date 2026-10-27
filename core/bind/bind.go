// Package bind 是 core 的 gomobile 绑定面：只跨 JSON 字符串、数值与 error。
// gobind 对返回值形状有硬限制（第二返回值必须是 error；不支持指针结构体与
// 二维切片），桌面用的 Go API 在这里包一层 JSON 入口；语义与 core 完全一致。
package bind

import (
	"encoding/json"
	"fmt"
	"time"

	"zakki/core"
)

// ParseTimeToken 解析行内第一个时间令牌簇（@时刻 + 可选 ~提前量 + 可选 ^规则），
// 返回 TimeTokenInfo 的 JSON：{start,end,due,dueMs,alertMs,hasAlert,recur}。
func ParseTimeToken(line string) (string, error) { return core.ParseTimeToken(line) }

// ParseRule 解析 ^ 规则段（如 "^w@1-5"），返回 Rule 的 JSON。
func ParseRule(s string) (string, error) { return core.ParseRule(s) }

// Expand 从锚点展开未来实例，返回 Unix 毫秒数组 JSON（升序，不超过 maxCount）。
func Expand(ruleJSON string, anchorMs, nowMs int64, horizonDays, maxCount int) (string, error) {
	return core.Expand(ruleJSON, anchorMs, nowMs, horizonDays, maxCount)
}

// timeSuffixJSON 是 TimeSuffix 的 JSON 形状。AlertMs 不在档位白名单内时写回
// 会按无提醒处理（白名单：0m/5m/10m/30m/1h/1d）。
type timeSuffixJSON struct {
	Due      string `json:"due"`     // YYYY-MM-DD HH:mm
	AlertMs  int64  `json:"alertMs"` // 提前量毫秒；0 且 hasAlert 表示准点
	HasAlert bool   `json:"hasAlert"`
	Recur    string `json:"recur,omitempty"` // ^ 规则原文；空表示不重复
}

func parseSuffixJSON(suffixJSON string) (*core.TimeSuffix, error) {
	if suffixJSON == "" {
		return nil, nil
	}
	var j timeSuffixJSON
	if err := json.Unmarshal([]byte(suffixJSON), &j); err != nil {
		return nil, fmt.Errorf("suffix JSON 非法: %w", err)
	}
	due, err := time.ParseInLocation("2006-01-02 15:04", j.Due, time.Local)
	if err != nil {
		return nil, fmt.Errorf("due 非法: %w", err)
	}
	return &core.TimeSuffix{Time: due, Alert: time.Duration(j.AlertMs) * time.Millisecond, HasAlert: j.HasAlert, Recur: j.Recur}, nil
}

func suffixToJSON(lt *core.TimeSuffix) (*timeSuffixJSON, error) {
	if lt == nil {
		return nil, nil
	}
	return &timeSuffixJSON{
		Due:      lt.Time.Format("2006-01-02 15:04"),
		AlertMs:  int64(lt.Alert),
		HasAlert: lt.HasAlert,
		Recur:    lt.Recur,
	}, nil
}

// ReplaceTimeTokenJSON 用 suffixJSON（timeSuffixJSON 形状；空串表示清除时间）
// 原位更新或清除行内时间令牌簇，返回新行文本。
func ReplaceTimeTokenJSON(line, suffixJSON string) (string, error) {
	lt, err := parseSuffixJSON(suffixJSON)
	if err != nil {
		return "", err
	}
	return core.ReplaceTimeTokenEx(line, lt, nil), nil
}

// SplitTimeSuffixJSON 剥离 line 行尾的时间令牌簇，返回
// {"text": 正文, "suffix": {…}|null} 的 JSON。
func SplitTimeSuffixJSON(line string) (string, error) {
	text, lt := core.SplitTimeSuffix(line)
	sj, err := suffixToJSON(lt)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(struct {
		Text   string          `json:"text"`
		Suffix *timeSuffixJSON `json:"suffix"`
	}{Text: text, Suffix: sj})
	return string(out), err
}

// JoinTimeSuffixJSON 把 suffixJSON（timeSuffixJSON 形状；空串表示去掉时间）
// 按规范格式写回 text 行尾，返回新行文本。
func JoinTimeSuffixJSON(text, suffixJSON string) (string, error) {
	lt, err := parseSuffixJSON(suffixJSON)
	if err != nil {
		return "", err
	}
	return core.JoinTimeSuffix(text, lt), nil
}
