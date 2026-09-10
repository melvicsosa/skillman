//go:build !windows

package cli

import "syscall"

// detachedAttr starts a child in its own session so it outlives the CLI.
func detachedAttr() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setsid: true} }
