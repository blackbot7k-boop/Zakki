package app

import "strings"

const liveHistorySnapshotLimit = 64 << 10

// 差分发布后不可修改：标签会话按值复制 liveEdit，共享这个指针。
// 边界为字节（允许落在 rune 内），选区仍由 liveSnapshot 保存 rune 坐标。
type liveEditDelta struct {
	prefix, suffix    int
	removed, inserted string
}

func (e liveEdit) compressed() liveEdit {
	if e.delta != nil || max(len(e.before.doc), len(e.after.doc)) < liveHistorySnapshotLimit {
		return e
	}
	before, after := e.before.doc, e.after.doc
	prefix, limit := 0, min(len(before), len(after))
	const chunk = 4096
	for prefix+chunk <= limit && before[prefix:prefix+chunk] == after[prefix:prefix+chunk] {
		prefix += chunk
	}
	for prefix < limit && before[prefix] == after[prefix] {
		prefix++
	}
	suffix := 0
	for suffix+chunk <= limit-prefix && before[len(before)-suffix-chunk:len(before)-suffix] == after[len(after)-suffix-chunk:len(after)-suffix] {
		suffix += chunk
	}
	for suffix < limit-prefix && before[len(before)-1-suffix] == after[len(after)-1-suffix] {
		suffix++
	}
	// 全文替换缺少共享区，克隆差分会打破相邻快照共享并反而放大内存。
	changed := len(before) + len(after) - 2*(prefix+suffix)
	if changed >= min(len(before), len(after))/2 {
		return e
	}
	e.delta = &liveEditDelta{
		prefix: prefix, suffix: suffix,
		removed:  strings.Clone(before[prefix : len(before)-suffix]),
		inserted: strings.Clone(after[prefix : len(after)-suffix]),
	}
	e.before.doc, e.after.doc = "", ""
	return e
}

func (e liveEdit) matchesAfter(doc string) bool {
	return e.delta == nil && e.after.doc == doc
}

// 每个非空栈的栈顶必须保留完整快照，入口据此逐字节校验外部改写。
// 压缩项只用刚验证过的相邻事务快照展开为新栈顶，不接受任意当前文本。
// 因此无须逐次计算整篇指纹，也不会漏掉公共前后缀上的外部修改。
func (e liveEdit) expanded(current string, redo bool) (liveEdit, bool) {
	if e.delta == nil {
		want := e.after.doc
		if redo {
			want = e.before.doc
		}
		return e, current == want
	}
	d := e.delta
	fragment, replaced := d.removed, d.inserted
	if redo {
		fragment, replaced = d.inserted, d.removed
	}
	if len(current) != d.prefix+len(replaced)+d.suffix || current[d.prefix:len(current)-d.suffix] != replaced {
		return e, false
	}
	restored := current[:d.prefix] + fragment + current[len(current)-d.suffix:]
	if redo {
		e.before.doc, e.after.doc = current, restored
	} else {
		e.before.doc, e.after.doc = restored, current
	}
	e.delta = nil
	return e, true
}
