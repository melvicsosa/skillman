package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/melvicsosa/skillman/internal/adapters/agents"
	"github.com/melvicsosa/skillman/internal/adapters/convert"
	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/adapters/storage/sqlite"
	"github.com/melvicsosa/skillman/internal/app"
	"github.com/melvicsosa/skillman/internal/domain"
)

func newTestServer(t *testing.T) (*httptest.Server, string) {
	srv, home, _ := newTestServerWithRegistry(t)
	return srv, home
}

func newTestServerWithRegistry(t *testing.T) (*httptest.Server, string, *fakeRegistry) {
	t.Helper()
	reg := &fakeRegistry{sources: map[string]string{}}
	home := t.TempDir()
	t.Setenv(agents.HomeEnv, home)
	dataDir := filepath.Join(t.TempDir(), "data")
	db, err := sqlite.Open(context.Background(), filepath.Join(dataDir, "skillman.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	svc := app.New(app.Deps{
		Agents: agents.Source{}, AgentRepo: sqlite.NewAgentRepo(db), Skills: sqlite.NewSkillRepo(db),
		Projects: sqlite.NewProjectRepo(db), Scans: sqlite.NewScanRepo(db),
		Inspector: skillfs.Inspector{}, Quarantine: skillfs.Mover{}, Vault: sqlite.NewVaultRepo(db),
		Settings: sqlite.NewSettingsRepo(db), Converter: convert.Converter{}, Registries: []domain.Registry{reg},
		DataDir: dataDir,
	})
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, home, reg
}

func writeSkill(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: d\n---\n"
	if err := os.WriteFile(filepath.Join(p, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func do(t *testing.T, method, url string, body any) (int, []byte) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func TestStaticAndHealthRoutes(t *testing.T) {
	srv, _ := newTestServer(t)
	status, body := do(t, "GET", srv.URL+"/api/health", nil)
	var health map[string]string
	if status != 200 || json.Unmarshal(body, &health) != nil || health["status"] != "ok" {
		t.Fatalf("health %d %s", status, body)
	}
	status, body = do(t, "GET", srv.URL+"/", nil)
	if status != 200 || !strings.Contains(string(body), "<html") {
		t.Fatalf("index %d %s", status, body)
	}
	status, body2 := do(t, "GET", srv.URL+"/some/client/route", nil)
	if status != 200 || !bytes.Equal(body, body2) {
		t.Fatalf("SPA fallback %d", status)
	}
	status, body = do(t, "GET", srv.URL+"/api/missing", nil)
	var e map[string]string
	if status != 404 || json.Unmarshal(body, &e) != nil || e["error"] == "" {
		t.Fatalf("unknown api route %d %s", status, body)
	}
}

func TestAgentsSkillsScanEnableDisable(t *testing.T) {
	srv, home := newTestServer(t)
	alpha := writeSkill(t, filepath.Join(home, ".claude", "skills"), "alpha")
	writeSkill(t, filepath.Join(home, ".agents", "skills"), "beta")
	writeSkill(t, filepath.Join(home, ".claude", "plugins", "cache", "m", "p", "1.0", "skills"), "plug")

	status, body := do(t, "GET", srv.URL+"/api/agents", nil)
	var agentList []AgentView
	if status != 200 || json.Unmarshal(body, &agentList) != nil || len(agentList) != 7 {
		t.Fatalf("agents %d %s", status, body)
	}

	status, body = do(t, "POST", srv.URL+"/api/scan", nil)
	var sum app.ScanSummary
	if status != 200 || json.Unmarshal(body, &sum) != nil || sum.Found != 5 {
		t.Fatalf("scan %d %s", status, body)
	}

	status, body = do(t, "GET", srv.URL+"/api/skills?scope=global&agent=claude-code", nil)
	var skills []SkillView
	if status != 200 || json.Unmarshal(body, &skills) != nil || len(skills) != 1 || skills[0].Name != "alpha" {
		t.Fatalf("skills %d %s", status, body)
	}
	status, body = do(t, "GET", srv.URL+"/api/skills?q=bet", nil)
	if status != 200 || json.Unmarshal(body, &skills) != nil || len(skills) != 3 {
		t.Fatalf("skills q %d %s", status, body)
	}

	// Disable alpha through the API and check the filesystem.
	status, body = do(t, "GET", srv.URL+"/api/skills?agent=claude-code", nil)
	_ = json.Unmarshal(body, &skills)
	id := skills[0].ID
	status, body = do(t, "POST", srv.URL+"/api/skills/"+id+"/disable", nil)
	var sk SkillView
	if status != 200 || json.Unmarshal(body, &sk) != nil || sk.State != "disabled" || sk.QuarantinePath == "" {
		t.Fatalf("disable %d %s", status, body)
	}
	if _, err := os.Lstat(alpha); err == nil {
		t.Fatal("alpha still in agent dir")
	}
	status, body = do(t, "POST", srv.URL+"/api/skills/"+id+"/enable", nil)
	if status != 200 || json.Unmarshal(body, &sk) != nil || sk.State != "enabled" {
		t.Fatalf("enable %d %s", status, body)
	}
	if _, err := os.Stat(filepath.Join(alpha, "SKILL.md")); err != nil {
		t.Fatal("alpha not restored")
	}

	// Read-only plugin skills cannot be disabled.
	status, body = do(t, "GET", srv.URL+"/api/skills?agent=claude", nil)
	_ = json.Unmarshal(body, &skills)
	status, body = do(t, "POST", srv.URL+"/api/skills/"+skills[0].ID+"/disable", nil)
	var e map[string]string
	if status != 409 || json.Unmarshal(body, &e) != nil || !strings.Contains(e["error"], "read-only") {
		t.Fatalf("disable read-only %d %s", status, body)
	}
	status, body = do(t, "POST", srv.URL+"/api/skills/nope/enable", nil)
	if status != 404 {
		t.Fatalf("enable unknown %d %s", status, body)
	}

	// Agent toggle.
	status, body = do(t, "PATCH", srv.URL+"/api/agents/opencode", map[string]bool{"enabled": false})
	var av AgentView
	if status != 200 || json.Unmarshal(body, &av) != nil || av.Enabled {
		t.Fatalf("patch agent %d %s", status, body)
	}
	status, _ = do(t, "PATCH", srv.URL+"/api/agents/opencode", map[string]string{"x": "y"})
	if status != 400 {
		t.Fatalf("patch agent bad body %d", status)
	}
	status, _ = do(t, "PATCH", srv.URL+"/api/agents/nope", map[string]bool{"enabled": true})
	if status != 404 {
		t.Fatalf("patch unknown agent %d", status)
	}

	// Doctor.
	status, body = do(t, "GET", srv.URL+"/api/doctor", nil)
	var report app.DoctorReport
	if status != 200 || json.Unmarshal(body, &report) != nil || report.Issues == nil {
		t.Fatalf("doctor %d %s", status, body)
	}
}

func TestProjectsRoutes(t *testing.T) {
	srv, home := newTestServer(t)
	root := filepath.Join(home, "proj")
	writeSkill(t, filepath.Join(root, ".agents", "skills"), "shared")

	status, body := do(t, "POST", srv.URL+"/api/projects", map[string]string{"root": root})
	var created struct {
		Project ProjectView     `json:"project"`
		Scan    app.ScanSummary `json:"scan"`
	}
	if status != 201 || json.Unmarshal(body, &created) != nil || created.Project.Name != "proj" || created.Scan.Found == 0 {
		t.Fatalf("add project %d %s", status, body)
	}
	status, _ = do(t, "POST", srv.URL+"/api/projects", map[string]string{"root": root})
	if status != 409 {
		t.Fatalf("duplicate project %d", status)
	}
	status, _ = do(t, "POST", srv.URL+"/api/projects", map[string]string{"root": filepath.Join(home, "missing")})
	if status != 400 {
		t.Fatalf("missing project %d", status)
	}
	status, body = do(t, "GET", srv.URL+"/api/skills?scope=project&project="+root, nil)
	var skills []SkillView
	if status != 200 || json.Unmarshal(body, &skills) != nil || len(skills) == 0 || skills[0].ProjectRoot != root {
		t.Fatalf("project skills %d %s", status, body)
	}
	status, body = do(t, "GET", srv.URL+"/api/skills?scope=project", nil)
	if status != 200 || json.Unmarshal(body, &skills) != nil || len(skills) == 0 {
		t.Fatalf("all project skills %d %s", status, body)
	}
	status, body = do(t, "GET", srv.URL+"/api/projects", nil)
	var projects []ProjectView
	if status != 200 || json.Unmarshal(body, &projects) != nil || len(projects) != 1 {
		t.Fatalf("list projects %d %s", status, body)
	}
	status, _ = do(t, "DELETE", srv.URL+"/api/projects/"+strconv.FormatInt(projects[0].ID, 10), nil)
	if status != 204 {
		t.Fatalf("delete project %d", status)
	}
	status, _ = do(t, "DELETE", srv.URL+"/api/projects/999", nil)
	if status != 404 {
		t.Fatalf("delete unknown project %d", status)
	}
}
