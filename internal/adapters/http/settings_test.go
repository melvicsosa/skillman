package http

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/melvicsosa/skillman/internal/domain"
)

// fakeManager is an in-memory domain.ServiceManager for handler tests.
type fakeManager struct {
	installed, running bool
	restarts           int
}

func (m *fakeManager) Install(context.Context, domain.ServiceSpec) error {
	m.installed, m.running = true, true
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
	return domain.ServiceState{Supported: true, Platform: "test", Label: domain.ServiceLabel, Installed: m.installed, Running: m.running}, nil
}

func decode(t *testing.T, body []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
}

func TestSettingsAPI(t *testing.T) {
	srv, _ := newTestServer(t)

	var got map[string]any
	_, body := do(t, http.MethodGet, srv.URL+"/api/settings", nil)
	decode(t, body, &got)
	if got["port"].(float64) != 3010 || got["hasGithubToken"] != false || got["dataDir"] == "" {
		t.Fatalf("defaults = %v", got)
	}

	if status, _ := do(t, http.MethodPatch, srv.URL+"/api/settings", map[string]any{"port": 22}); status != http.StatusUnprocessableEntity {
		t.Fatalf("privileged port status = %d", status)
	}

	got = nil
	_, body = do(t, http.MethodPatch, srv.URL+"/api/settings", map[string]any{"port": 3222, "githubToken": "secret"})
	decode(t, body, &got)
	if got["port"].(float64) != 3222 || got["hasGithubToken"] != true {
		t.Fatalf("patched = %v", got)
	}
	if _, leaked := got["githubToken"]; leaked {
		t.Fatal("token value returned")
	}
	if _, ok := got["restartRequired"]; ok {
		t.Fatal("restartRequired without a running service")
	}

	// With the service running, a port change asks for a restart.
	if status, _ := do(t, http.MethodPost, srv.URL+"/api/service/install", nil); status != http.StatusOK {
		t.Fatalf("install status = %d", status)
	}
	got = nil
	_, body = do(t, http.MethodPatch, srv.URL+"/api/settings", map[string]any{"port": 3333})
	decode(t, body, &got)
	if got["restartRequired"] != true {
		t.Fatalf("restartRequired missing: %v", got)
	}
}

func TestServiceAPI(t *testing.T) {
	srv, _ := newTestServer(t)

	var info map[string]any
	_, body := do(t, http.MethodGet, srv.URL+"/api/service", nil)
	decode(t, body, &info)
	if info["supported"] != true || info["installed"] != false || info["healthy"] != false || info["port"].(float64) != 3010 {
		t.Fatalf("status = %v", info)
	}

	if status, _ := do(t, http.MethodPost, srv.URL+"/api/service/restart", nil); status != http.StatusConflict {
		t.Fatalf("restart before install = %d", status)
	}

	info = nil
	_, body = do(t, http.MethodPost, srv.URL+"/api/service/install", nil)
	decode(t, body, &info)
	if info["installed"] != true || info["running"] != true {
		t.Fatalf("after install = %v", info)
	}
	if status, _ := do(t, http.MethodPost, srv.URL+"/api/service/restart", nil); status != http.StatusAccepted {
		t.Fatalf("restart = %d", status)
	}
	info = nil
	_, body = do(t, http.MethodPost, srv.URL+"/api/service/uninstall", nil)
	decode(t, body, &info)
	if info["installed"] != false {
		t.Fatalf("after uninstall = %v", info)
	}
}

func TestUsageAPIAndSkillViews(t *testing.T) {
	srv, home := newTestServer(t)
	writeSkill(t, filepath.Join(home, ".claude", "skills"), "branch-pr")
	writeSkill(t, filepath.Join(home, ".agents", "skills"), "branch-pr")
	usage := `{"skillUsage":{"branch-pr":{"usageCount":10,"lastUsedAt":1788913317724}}}`
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(usage), 0o600); err != nil {
		t.Fatal(err)
	}
	do(t, http.MethodPost, srv.URL+"/api/scan", nil)

	var report struct {
		Source string                       `json:"source"`
		Skills map[string]domain.SkillUsage `json:"skills"`
	}
	_, body := do(t, http.MethodGet, srv.URL+"/api/usage", nil)
	decode(t, body, &report)
	if !strings.HasSuffix(report.Source, ".claude.json") || report.Skills["branch-pr"].UsageCount != 10 {
		t.Fatalf("usage = %+v", report)
	}

	var skills []SkillView
	_, body = do(t, http.MethodGet, srv.URL+"/api/skills", nil)
	decode(t, body, &skills)
	byAgent := map[string]SkillView{}
	for _, s := range skills {
		byAgent[s.Agent] = s
	}
	if cc := byAgent["claude-code"]; cc.UsageCount != 10 || !strings.HasPrefix(cc.LastUsedAt, "2026-") {
		t.Fatalf("claude-code view = %+v", cc)
	}
	if cx := byAgent["codex"]; cx.UsageCount != 0 || cx.LastUsedAt != "" {
		t.Fatalf("codex view = %+v", cx)
	}
}
