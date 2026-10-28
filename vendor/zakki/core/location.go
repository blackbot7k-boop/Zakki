package core

import (
	"fmt"
	"sync/atomic"
	"time"
)

// RuleSyntaxVersion 标识当前重复规则语法版本。
const RuleSyntaxVersion = 1

var selectedLocation atomic.Pointer[time.Location]

// Location 返回当前共享内核的时区。
func Location() *time.Location { return location() }

func location() *time.Location {
	if loc := selectedLocation.Load(); loc != nil {
		return loc
	}
	return time.Local
}

// SetLocation 原子切换内核使用的时区；空名称恢复系统本地时区。
func SetLocation(name string) error {
	if name == "" {
		selectedLocation.Store(nil)
		return nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return fmt.Errorf("时区非法: %w", err)
	}
	selectedLocation.Store(loc)
	return nil
}
