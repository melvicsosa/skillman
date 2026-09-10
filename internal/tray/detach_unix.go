//go:build !windows

package tray

import "syscall"

// detachedAttr puts the child in its own session so it survives the tray.
func detachedAttr() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setsid: true} }
