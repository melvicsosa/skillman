//go:build !darwin

package tray

import "errors"

// ErrUnsupported is returned by Run outside macOS.
var ErrUnsupported = errors.New("skillman-tray is macOS only")

// Run is only implemented on macOS.
func Run(Config) error { return ErrUnsupported }
