package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/melvicsosa/skillman/internal/brewpath"
	"github.com/melvicsosa/skillman/internal/domain"
)

// ServiceEnv is set in the LaunchAgent environment so a server can tell it
// runs under the service manager.
const ServiceEnv = "SKILLMAN_SERVICE"

// PIDFileName is written next to the database by `serve`.
const PIDFileName = "serve.pid"

// ServiceInfo is the full picture of the background service: what the
// platform manager reports plus what the server on the configured port says.
type ServiceInfo struct {
	domain.ServiceState
	Port         int    `json:"port"`
	URL          string `json:"url"`
	Healthy      bool   `json:"healthy"`
	ServerPID    int    `json:"serverPid,omitempty"`
	Executable   string `json:"executable,omitempty"`
	DataDir      string `json:"dataDir"`
	UnderService bool   `json:"underService"`
}

// HealthProbe reports whether a skillman server answers on port and its pid.
type HealthProbe func(ctx context.Context, port int) (healthy bool, pid int)

// UnderService reports whether this process was started by the service manager.
func UnderService() bool { return os.Getenv(ServiceEnv) != "" }

// PIDFile is <dataDir>/serve.pid.
func (s *Service) PIDFile() string { return filepath.Join(s.dataDir, PIDFileName) }

// DataDir returns the data directory.
func (s *Service) DataDir() string { return s.dataDir }

func (s *Service) manager() (domain.ServiceManager, error) {
	if s.serviceMgr == nil {
		return nil, fmt.Errorf("background service: %w", domain.ErrUnsupported)
	}
	return s.serviceMgr, nil
}

// ServiceStatus never fails because the platform is unsupported; it reports
// Supported=false instead so the UI can hide the controls.
func (s *Service) ServiceStatus(ctx context.Context) (ServiceInfo, error) {
	info := ServiceInfo{DataDir: s.dataDir, UnderService: UnderService()}
	info.Label = domain.ServiceLabel
	if s.serviceMgr != nil {
		st, err := s.serviceMgr.Status(ctx)
		if err != nil {
			return info, err
		}
		info.ServiceState = st
	}
	port, err := s.Port(ctx)
	if err != nil {
		return info, err
	}
	info.Port = port
	info.URL = "http://localhost:" + strconv.Itoa(port)
	if exe, err := os.Executable(); err == nil {
		info.Executable = exe
	}
	info.Healthy, info.ServerPID = s.probe(ctx, port)
	if info.ServerPID == 0 {
		info.ServerPID = ReadPIDFile(s.PIDFile())
	}
	return info, nil
}

// ServiceInstall registers the current executable as a login service.
func (s *Service) ServiceInstall(ctx context.Context) (ServiceInfo, error) {
	mgr, err := s.manager()
	if err != nil {
		return ServiceInfo{}, err
	}
	exe, err := os.Executable()
	if err != nil {
		return ServiceInfo{}, fmt.Errorf("resolve executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	// Persist the Homebrew opt link, not the versioned Cellar path that
	// `brew upgrade` deletes.
	exe = brewpath.Stable(exe)
	spec := domain.ServiceSpec{
		Executable: exe,
		Args:       []string{"--data-dir", s.dataDir, "serve", "--no-open"},
		Env:        map[string]string{"PATH": servicePath(), ServiceEnv: "launchd"},
	}
	for _, k := range []string{"SKILLMAN_HOME", "HOME"} {
		if v := os.Getenv(k); v != "" {
			spec.Env[k] = v
		}
	}
	if err := mgr.Install(ctx, spec); err != nil {
		return ServiceInfo{}, err
	}
	return s.ServiceStatus(ctx)
}

func servicePath() string {
	if p := os.Getenv("PATH"); p != "" {
		return p
	}
	return "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin"
}

// ServiceUninstall stops and removes the login service.
func (s *Service) ServiceUninstall(ctx context.Context) (ServiceInfo, error) {
	mgr, err := s.manager()
	if err != nil {
		return ServiceInfo{}, err
	}
	if err := mgr.Uninstall(ctx); err != nil {
		return ServiceInfo{}, err
	}
	return s.ServiceStatus(ctx)
}

// ServiceStart starts the installed service.
func (s *Service) ServiceStart(ctx context.Context) error {
	mgr, err := s.manager()
	if err != nil {
		return err
	}
	return mgr.Start(ctx)
}

// ServiceStop stops the installed service.
func (s *Service) ServiceStop(ctx context.Context) error {
	mgr, err := s.manager()
	if err != nil {
		return err
	}
	return mgr.Stop(ctx)
}

// ServiceRestart asks the platform manager to kill and relaunch the service.
// Under launchd this is `launchctl kickstart -k`, which works even when the
// caller is the instance being restarted: launchd sends SIGTERM, the server
// shuts down cleanly, and launchd spawns the new one (which then reads the
// current port setting).
func (s *Service) ServiceRestart(ctx context.Context) error {
	mgr, err := s.manager()
	if err != nil {
		return err
	}
	return mgr.Restart(ctx)
}

// ProbeHealth asks the server on port for /api/health and returns whether it
// answered and the pid it reported.
func ProbeHealth(ctx context.Context, port int) (bool, int) {
	ctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+"/api/health", nil)
	if err != nil {
		return false, 0
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, 0
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return false, 0
	}
	var body struct {
		Status string `json:"status"`
		PID    int    `json:"pid"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil || body.Status != "ok" {
		return false, 0
	}
	return true, body.PID
}

// ReadPIDFile returns the pid recorded in p, or 0.
func ReadPIDFile(p string) int {
	raw, err := os.ReadFile(p)
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(string(bytesTrim(raw)))
	return pid
}

func bytesTrim(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r' || b[len(b)-1] == ' ') {
		b = b[:len(b)-1]
	}
	return b
}
