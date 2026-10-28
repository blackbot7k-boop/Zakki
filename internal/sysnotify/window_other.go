//go:build !windows

package sysnotify

import "os/exec"

func hideNotificationWindow(*exec.Cmd) {}
