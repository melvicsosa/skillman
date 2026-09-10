package domain

import (
	"context"
	"time"
)

// ServiceLabel is the launchd/systemd identifier of the background server.
const ServiceLabel = "com.melvicsosa.skillman"

// ServiceState is what the platform service manager knows about the
// background server: whether the unit is installed and whether it runs.
type ServiceState struct {
	Supported bool   `json:"supported"`
	Platform  string `json:"platform"`
	Label     string `json:"label"`
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	PID       int    `json:"pid,omitempty"`
	// UnitPath is the plist (or unit file) location.
	UnitPath string `json:"unitPath,omitempty"`
	LogDir   string `json:"logDir,omitempty"`
}

// ServiceSpec describes the process the service manager should keep alive.
type ServiceSpec struct {
	Executable string
	Args       []string
	Env        map[string]string
}

// ServiceManager installs the server as a login service. Operations that
// the platform cannot perform return an error wrapping ErrUnsupported.
type ServiceManager interface {
	Install(ctx context.Context, spec ServiceSpec) error
	Uninstall(ctx context.Context) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
	Status(ctx context.Context) (ServiceState, error)
}

// SkillUsage is one agent's usage telemetry for a skill.
type SkillUsage struct {
	UsageCount int       `json:"usageCount"`
	LastUsedAt time.Time `json:"lastUsedAt"`
}

// UsageSource reads skill usage telemetry an agent records on its own
// (Claude Code keeps it in ~/.claude.json under skillUsage).
type UsageSource interface {
	// Usage returns the telemetry keyed by the agent's own skill key and the
	// file it came from. A missing file yields an empty map, not an error.
	Usage() (map[string]SkillUsage, string, error)
}
