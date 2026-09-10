//go:build windows

package tray

import "syscall"

func detachedAttr() *syscall.SysProcAttr { return nil }
