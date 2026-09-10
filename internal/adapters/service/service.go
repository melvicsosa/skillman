// Package service installs the skillman server as a login service. macOS
// uses a launchd LaunchAgent; other platforms report ErrUnsupported.
package service

import (
	"context"
	"fmt"
	"runtime"

	"github.com/melvicsosa/skillman/internal/domain"
)

// New returns the service manager for the current platform. plistDir and
// dataDir are explicit so tests can point them at temporary directories.
func New(plistDir, dataDir string) domain.ServiceManager {
	if runtime.GOOS == "darwin" {
		return NewLaunchd(plistDir, dataDir)
	}
	return Unsupported{Platform: runtime.GOOS}
}

// Unsupported is the manager for platforms without an implementation.
type Unsupported struct{ Platform string }

var _ domain.ServiceManager = Unsupported{}

func (u Unsupported) err() error {
	return fmt.Errorf("background service on %s: %w (only macOS launchd is implemented)", u.Platform, domain.ErrUnsupported)
}

// Install implements domain.ServiceManager.
func (u Unsupported) Install(context.Context, domain.ServiceSpec) error { return u.err() }

// Uninstall implements domain.ServiceManager.
func (u Unsupported) Uninstall(context.Context) error { return u.err() }

// Start implements domain.ServiceManager.
func (u Unsupported) Start(context.Context) error { return u.err() }

// Stop implements domain.ServiceManager.
func (u Unsupported) Stop(context.Context) error { return u.err() }

// Restart implements domain.ServiceManager.
func (u Unsupported) Restart(context.Context) error { return u.err() }

// Status implements domain.ServiceManager.
func (u Unsupported) Status(context.Context) (domain.ServiceState, error) {
	return domain.ServiceState{Supported: false, Platform: u.Platform, Label: domain.ServiceLabel}, nil
}
