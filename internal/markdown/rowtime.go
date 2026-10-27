package markdown

import "zakki/core"

// 时间令牌的语义实现在共享内核 zakki/core（与未来移动端 gomobile 同源）；
// 本文件只保留桌面的调用面。行尾解析与档位白名单的规则说明见 core/timetoken.go。

// TimeSuffix 是一行行尾解析出的时间与提醒（含可选重复规则）。
type TimeSuffix = core.TimeSuffix

// AlertTier 是提醒提前量档位（弹层展示与解析共用一份白名单）。
type AlertTier = core.AlertTier

// AlertTiers 返回全部提醒档位，顺序即弹层展示顺序。
func AlertTiers() []AlertTier { return core.AlertTiers() }

// SplitTimeSuffix 剥离 line 行尾的时间令牌簇，返回正文与解析结果；行尾没有
// 合法令牌时原样返回 line 与 nil。
func SplitTimeSuffix(line string) (string, *TimeSuffix) { return core.SplitTimeSuffix(line) }

// JoinTimeSuffix 把时间令牌簇按规范格式写回行尾；lt 为 nil 表示去掉时间。
func JoinTimeSuffix(text string, lt *TimeSuffix) string { return core.JoinTimeSuffix(text, lt) }
