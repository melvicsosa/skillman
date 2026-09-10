//go:build windows

package cli

import "syscall"

func detachedAttr() *syscall.SysProcAttr { return nil }
