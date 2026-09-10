package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/melvicsosa/skillman/internal/domain"
)

func TestSettingsPortAndTokens(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	st, err := f.svc.GetSettings(ctx)
	if err != nil || st.Port != DefaultPort || st.HasGitHubToken || st.HasSkillsSHToken {
		t.Fatalf("defaults = %+v, %v", st, err)
	}

	port := 80
	if _, _, err := f.svc.UpdateSettings(ctx, SettingsPatch{Port: &port}); !errors.Is(err, domain.ErrRejected) {
		t.Fatalf("privileged port accepted: %v", err)
	}
	port = 4321
	tok := "ghp_x"
	st, changed, err := f.svc.UpdateSettings(ctx, SettingsPatch{Port: &port, GitHubToken: &tok})
	if err != nil || !changed || st.Port != 4321 || !st.HasGitHubToken || st.HasSkillsSHToken {
		t.Fatalf("update = %+v, changed=%v, %v", st, changed, err)
	}
	if _, changed, _ = f.svc.UpdateSettings(ctx, SettingsPatch{Port: &port}); changed {
		t.Fatal("same port reported as changed")
	}
	empty := ""
	st, _, _ = f.svc.UpdateSettings(ctx, SettingsPatch{GitHubToken: &empty})
	if st.HasGitHubToken {
		t.Fatal("empty token did not clear")
	}

	if v, err := f.svc.ConfigGet(ctx, "port"); err != nil || v != "4321" {
		t.Fatalf("config get port = %q, %v", v, err)
	}
	if err := f.svc.ConfigSet(ctx, "port", "abc"); !errors.Is(err, domain.ErrRejected) {
		t.Fatalf("non-numeric port: %v", err)
	}
	if err := f.svc.ConfigSet(ctx, "nope", "1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown key: %v", err)
	}
	if err := f.svc.ConfigSet(ctx, "skillssh_token", " tok "); err != nil {
		t.Fatal(err)
	}
	if v, _ := f.svc.ConfigGet(ctx, "skillssh_token"); v != "tok" {
		t.Fatalf("token = %q", v)
	}

	// A corrupt stored port falls back to the default.
	if err := f.svc.settings.SetSetting(ctx, SettingPort, "garbage"); err != nil {
		t.Fatal(err)
	}
	if p, _ := f.svc.Port(ctx); p != DefaultPort {
		t.Fatalf("port = %d", p)
	}
}

func TestUsageFor(t *testing.T) {
	usage := map[string]domain.SkillUsage{
		"branch-pr":     {UsageCount: 10, LastUsedAt: time.Now()},
		"vercel:deploy": {UsageCount: 2},
		"init":          {UsageCount: 1},
	}
	if u, ok := UsageFor(usage, "claude-code", "branch-pr", "/h/.claude/skills/branch-pr"); !ok || u.UsageCount != 10 {
		t.Fatalf("claude-code: %+v %v", u, ok)
	}
	if _, ok := UsageFor(usage, "codex", "branch-pr", "/h/.agents/skills/branch-pr"); ok {
		t.Fatal("codex must not match")
	}
	p := "/h/.claude/plugins/cache/vercel-mp/vercel/1.0.0/skills/deploy"
	if u, ok := UsageFor(usage, "claude", "deploy", p); !ok || u.UsageCount != 2 {
		t.Fatalf("plugin: %+v %v", u, ok)
	}
	p = "/h/.claude/plugins/cache/mp/other/1.0.0/skills/init"
	if u, ok := UsageFor(usage, "claude", "init", p); !ok || u.UsageCount != 1 {
		t.Fatalf("plugin fallback by name: %+v %v", u, ok)
	}
	if _, ok := UsageFor(nil, "claude-code", "x", ""); ok {
		t.Fatal("nil usage matched")
	}
}

// fakeManager is an in-memory domain.ServiceManager.
type fakeManager struct {
	installed, running bool
	spec               domain.ServiceSpec
	restarts           int
}

func (m *fakeManager) Install(_ context.Context, spec domain.ServiceSpec) error {
	m.installed, m.running, m.spec = true, true, spec
	return nil
}
func (m *fakeManager) Uninstall(context.Context) error {
	m.installed, m.running = false, false
	return nil
}
func (m *fakeManager) Start(context.Context) error   { m.running = true; return nil }
func (m *fakeManager) Stop(context.Context) error    { m.running = false; return nil }
func (m *fakeManager) Restart(context.Context) error { m.restarts++; return nil }
func (m *fakeManager) Status(context.Context) (domain.ServiceState, error) {
	return domain.ServiceState{Supported: true, Platform: "test", Label: domain.ServiceLabel, Installed: m.installed, Running: m.running, PID: 7}, nil
}

func TestServiceLifecycle(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// Without a manager every mutation is unsupported but status still works.
	if _, err := f.svc.ServiceInstall(ctx); !errors.Is(err, domain.ErrUnsupported) {
		t.Fatalf("install without manager: %v", err)
	}
	info, err := f.svc.ServiceStatus(ctx)
	if err != nil || info.Supported || info.Port != DefaultPort || info.DataDir != f.dataDir || info.Healthy {
		t.Fatalf("status without manager = %+v, %v", info, err)
	}

	m := &fakeManager{}
	f.svc.serviceMgr = m
	if err := os.WriteFile(f.svc.PIDFile(), []byte("4242\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err = f.svc.ServiceInstall(ctx)
	if err != nil || !info.Installed || !info.Running || info.PID != 7 || info.ServerPID != 4242 {
		t.Fatalf("install = %+v, %v", info, err)
	}
	if m.spec.Args[0] != "--data-dir" || m.spec.Args[1] != f.dataDir || m.spec.Args[2] != "serve" || m.spec.Args[3] != "--no-open" {
		t.Fatalf("args = %v", m.spec.Args)
	}
	if m.spec.Env[ServiceEnv] == "" || m.spec.Env["PATH"] == "" || !filepath.IsAbs(m.spec.Executable) {
		t.Fatalf("spec = %+v", m.spec)
	}
	if err := f.svc.ServiceRestart(ctx); err != nil || m.restarts != 1 {
		t.Fatalf("restart: %v (%d)", err, m.restarts)
	}
	if err := f.svc.ServiceStop(ctx); err != nil || m.running {
		t.Fatal("stop")
	}
	if err := f.svc.ServiceStart(ctx); err != nil || !m.running {
		t.Fatal("start")
	}
	info, err = f.svc.ServiceUninstall(ctx)
	if err != nil || info.Installed {
		t.Fatalf("uninstall = %+v, %v", info, err)
	}
}
