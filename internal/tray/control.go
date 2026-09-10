package tray

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// pidFileName mirrors app.PIDFileName; the tray does not import internal/app
// so the core packages never pull tray code (and vice versa).
const pidFileName = "serve.pid"

// Status is what the menu shows: the service manager's view plus the
// answer of the server on the configured port.
type Status struct {
	Supported bool   `json:"supported"`
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	Port      int    `json:"port"`
	URL       string `json:"url"`
	Healthy   bool   `json:"healthy"`
	ServerPID int    `json:"serverPid"`
	DataDir   string `json:"dataDir"`
	// Version comes from GET /api/health when the server answers.
	Version string `json:"-"`
}

// Label is the status line shown at the top of the menu.
func (s Status) Label() string {
	switch {
	case s.Healthy:
		l := "Running on :" + strconv.Itoa(s.Port)
		if s.Version != "" {
			l += " · v" + strings.TrimPrefix(s.Version, "v")
		}
		return l
	case s.Installed:
		return "Stopped"
	default:
		return "Service not installed"
	}
}

// Controller drives the skillman server through the CLI binary.
type Controller struct {
	cfg Config
	// Timeout bounds every CLI call.
	Timeout time.Duration
	// HTTP probes /api/health.
	HTTP *http.Client
}

// NewController returns a controller for cfg.
func NewController(cfg Config) *Controller {
	return &Controller{cfg: cfg, Timeout: 8 * time.Second, HTTP: &http.Client{Timeout: 1500 * time.Millisecond}}
}

// Config returns the controller configuration.
func (c *Controller) Config() Config { return c.cfg }

// PIDFile is <DataDir>/serve.pid, written by `skillman serve`.
func (c *Controller) PIDFile() string { return filepath.Join(c.cfg.DataDir, pidFileName) }

// LogFile receives stdout and stderr of a server the tray spawns itself.
func (c *Controller) LogFile() string { return filepath.Join(c.cfg.DataDir, "logs", "skillman.log") }

func (c *Controller) run(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	full := append([]string{"--data-dir", c.cfg.DataDir}, args...)
	out, err := exec.CommandContext(ctx, c.cfg.Skillman, full...).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return string(out), fmt.Errorf("skillman %s: %s", strings.Join(args, " "), msg)
	}
	return string(out), nil
}

// Status asks `skillman service status --json` and the health endpoint.
func (c *Controller) Status(ctx context.Context) (Status, error) {
	out, err := c.run(ctx, "service", "status", "--json")
	if err != nil {
		return Status{}, err
	}
	st, err := parseStatus(out)
	if err != nil {
		return Status{}, err
	}
	if st.Healthy {
		st.Version, _ = c.health(ctx, st.Port)
	}
	return st, nil
}

func parseStatus(out string) (Status, error) {
	var st Status
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &st); err != nil {
		return Status{}, fmt.Errorf("parse service status: %w", err)
	}
	return st, nil
}

// health returns the version reported by /api/health on port.
func (c *Controller) health(ctx context.Context, port int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+"/api/health", nil)
	if err != nil {
		return "", err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("health: %s", res.Status)
	}
	var body struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil || body.Status != "ok" {
		return "", errors.New("health: unexpected answer")
	}
	return body.Version, nil
}

// Start starts the installed service, or spawns a detached `skillman serve`
// when no service is installed.
func (c *Controller) Start(ctx context.Context, st Status) error {
	if st.Installed {
		_, err := c.run(ctx, "service", "start")
		return err
	}
	if st.Healthy {
		return nil
	}
	return c.spawnServe()
}

// Stop stops the installed service, or terminates the detached server via
// the pid file.
func (c *Controller) Stop(ctx context.Context, st Status) error {
	if st.Installed {
		_, err := c.run(ctx, "service", "stop")
		return err
	}
	return c.killServe(st)
}

// Restart restarts the service, or kills and respawns the detached server.
func (c *Controller) Restart(ctx context.Context, st Status) error {
	if st.Installed {
		_, err := c.run(ctx, "service", "restart")
		return err
	}
	if err := c.killServe(st); err != nil {
		return err
	}
	return c.spawnServe()
}

// Install registers the skillman LaunchAgent (`service install`).
func (c *Controller) Install(ctx context.Context) error {
	_, err := c.run(ctx, "service", "install")
	return err
}

// Uninstall removes the LaunchAgent (`service uninstall`).
func (c *Controller) Uninstall(ctx context.Context) error {
	_, err := c.run(ctx, "service", "uninstall")
	return err
}

// SetPort stores the port setting and restarts whatever is running so it
// takes effect.
func (c *Controller) SetPort(ctx context.Context, st Status, port int) error {
	if _, err := c.run(ctx, "config", "set", "port", strconv.Itoa(port)); err != nil {
		return err
	}
	if !st.Installed && !st.Healthy {
		return nil
	}
	return c.Restart(ctx, st)
}

// spawnServe launches `skillman serve --no-open` in its own session so it
// outlives the tray, logging to <DataDir>/logs/skillman.log.
func (c *Controller) spawnServe() error {
	if err := os.MkdirAll(filepath.Dir(c.LogFile()), 0o755); err != nil {
		return err
	}
	logf, err := os.OpenFile(c.LogFile(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logf.Close()
	cmd := exec.Command(c.cfg.Skillman, "--data-dir", c.cfg.DataDir, "serve", "--no-open")
	cmd.Stdout = logf
	cmd.Stderr = logf
	cmd.SysProcAttr = detachedAttr()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start skillman serve: %w", err)
	}
	// Reap the child when it exits so the pid never lingers as a zombie
	// (which would make the liveness probe in killServe believe it runs).
	go func() { _ = cmd.Wait() }()
	return nil
}

// killServe sends SIGTERM to the server (health pid first, pid file second)
// and waits for it to exit.
func (c *Controller) killServe(st Status) error {
	pid := st.ServerPID
	if pid == 0 {
		pid = readPIDFile(c.PIDFile())
	}
	if pid <= 0 {
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return fmt.Errorf("stop pid %d: %w", pid, err)
	}
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if proc.Signal(syscall.Signal(0)) != nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("pid %d did not exit after SIGTERM", pid)
}

func readPIDFile(p string) int {
	raw, err := os.ReadFile(p)
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	return pid
}

// WaitHealthy polls the health endpoint until it answers or ctx ends.
func (c *Controller) WaitHealthy(ctx context.Context, port int, d time.Duration) bool {
	ctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	for {
		if _, err := c.health(ctx, port); err == nil {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(250 * time.Millisecond):
		}
	}
}
