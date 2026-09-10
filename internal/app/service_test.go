package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/melvicsosa/skillman/internal/adapters/agents"
	"github.com/melvicsosa/skillman/internal/adapters/convert"
	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/adapters/storage/sqlite"
	"github.com/melvicsosa/skillman/internal/domain"
)

// fixture is a fake home directory plus a data dir, wired to a Service.
type fixture struct {
	t       *testing.T
	home    string
	dataDir string
	db      *sqlite.DB
	svc     *Service
	reg     *fakeRegistry
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	home := t.TempDir()
	t.Setenv(agents.HomeEnv, home)
	f := &fixture{t: t, home: home, dataDir: filepath.Join(t.TempDir(), "data"), reg: &fakeRegistry{id: "fake", sources: map[string]string{}}}
	f.open()
	return f
}

func (f *fixture) open() {
	db, err := sqlite.Open(context.Background(), filepath.Join(f.dataDir, "skillman.db"))
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { db.Close() })
	f.db = db
	f.svc = New(Deps{
		Agents: agents.Source{}, AgentRepo: sqlite.NewAgentRepo(db), Skills: sqlite.NewSkillRepo(db),
		Projects: sqlite.NewProjectRepo(db), Scans: sqlite.NewScanRepo(db),
		Inspector: skillfs.Inspector{}, Quarantine: skillfs.Mover{}, Vault: sqlite.NewVaultRepo(db),
		Settings: sqlite.NewSettingsRepo(db), Converter: convert.Converter{}, Registries: []domain.Registry{f.reg},
		DataDir: f.dataDir,
	})
}

// wipeDB closes and deletes the database, then reopens a fresh one.
func (f *fixture) wipeDB() {
	f.db.Close()
	for _, suffix := range []string{"", "-wal", "-shm"} {
		os.Remove(filepath.Join(f.dataDir, "skillman.db"+suffix))
	}
	f.open()
}

func (f *fixture) path(rel string) string { return filepath.Join(f.home, rel) }

func (f *fixture) writeSkill(rel, name, body string) string {
	f.t.Helper()
	dir := filepath.Join(f.home, rel, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: Description of " + name + "\nmetadata:\n  version: \"1.0\"\n---\n" + body
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
	return dir
}

func (f *fixture) writeRaw(rel, name, content string) string {
	f.t.Helper()
	dir := filepath.Join(f.home, rel, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
	return dir
}

func (f *fixture) scan() ScanSummary {
	f.t.Helper()
	sum, err := f.svc.Scan(context.Background(), ScanOptions{})
	if err != nil {
		f.t.Fatalf("Scan: %v", err)
	}
	return sum
}

func (f *fixture) resolve(name string, agent domain.AgentID) domain.Skill {
	f.t.Helper()
	sk, err := f.svc.ResolveSkill(context.Background(), name, agent, "")
	if err != nil {
		f.t.Fatalf("ResolveSkill %s/%s: %v", agent, name, err)
	}
	return sk
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func TestScanDiscoversGlobalSkillsPerAgent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.writeSkill(".claude/skills", "alpha", "body A\n")
	f.writeSkill(".claude/skills", "beta", "body B\n")
	f.writeSkill(".agents/skills", "alpha", "body A\n")
	f.writeSkill(".config/opencode/skills", "gamma", "")
	f.writeSkill(".claude/plugins/cache/official/expo/1.0.0/skills", "eas", "")
	if err := os.MkdirAll(f.path(".claude/skills/_shared"), 0o755); err != nil {
		t.Fatal(err)
	}

	sum := f.scan()
	// claude-code: alpha, beta; codex/cursor/gemini-cli: alpha (~/.agents/skills); opencode: gamma; claude: eas.
	if sum.Found != 7 || sum.Added != 7 || sum.Removed != 0 || len(sum.Errors) != 0 {
		t.Fatalf("first scan = %+v", sum)
	}
	sum = f.scan()
	if sum.Found != 7 || sum.Added != 0 {
		t.Fatalf("second scan = %+v", sum)
	}

	list, err := f.svc.ListAgents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[domain.AgentID]int{}
	for _, a := range list {
		counts[a.ID] = a.SkillCount
	}
	want := map[domain.AgentID]int{"claude-code": 2, "claude": 1, "codex": 1, "cursor": 1, "gemini-cli": 1, "antigravity": 0, "opencode": 1}
	for id, n := range want {
		if counts[id] != n {
			t.Errorf("agent %s count = %d, want %d", id, counts[id], n)
		}
	}
	plugin := f.resolve("eas", "claude")
	if !plugin.ReadOnly || plugin.Version != "1.0" || plugin.Description != "Description of eas" {
		t.Fatalf("plugin skill = %+v", plugin)
	}
	if plugin.ID != domain.SkillID("claude", domain.GlobalScope, plugin.Path) {
		t.Fatalf("id is not derived from identity")
	}

	// Removal on disk shows up as removed.
	if err := os.RemoveAll(f.path(".claude/skills/beta")); err != nil {
		t.Fatal(err)
	}
	sum = f.scan()
	if sum.Found != 6 || sum.Removed != 1 {
		t.Fatalf("scan after removal = %+v", sum)
	}
	if _, err := f.svc.ResolveSkill(ctx, "beta", "claude-code", ""); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("beta should be gone, err = %v", err)
	}
}

func TestDisableEnableRoundTrip(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	src := f.writeSkill(".claude/skills", "alpha", "body\n")
	f.writeSkill(".claude/skills", "beta", "")
	f.scan()

	alpha := f.resolve("alpha", "claude-code")
	disabled, err := f.svc.DisableSkill(ctx, alpha.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantQ := filepath.Join(f.dataDir, "disabled", "claude-code", "alpha")
	if disabled.State != domain.SkillDisabled || disabled.QuarantinePath != wantQ {
		t.Fatalf("disabled = %+v", disabled)
	}
	if exists(src) {
		t.Fatal("skill still present in agent dir")
	}
	if !exists(filepath.Join(wantQ, "SKILL.md")) || !exists(wantQ+".origin.json") {
		t.Fatal("quarantine entry or sidecar missing")
	}
	found, _ := skillfs.DiscoverSkills(f.path(".claude/skills"))
	if len(found) != 1 || found[0].Name != "beta" {
		t.Fatalf("agent dir lists %+v", found)
	}

	// Disabling twice is a no-op.
	if again, err := f.svc.DisableSkill(ctx, alpha.ID); err != nil || again.QuarantinePath != wantQ {
		t.Fatalf("second disable: %+v %v", again, err)
	}

	// A rescan keeps the disabled row.
	sum := f.scan()
	if sum.Found != 1 || sum.Disabled != 1 || sum.Removed != 0 {
		t.Fatalf("rescan = %+v", sum)
	}
	got, _ := f.svc.GetSkill(ctx, alpha.ID)
	if got.State != domain.SkillDisabled || got.QuarantinePath != wantQ || got.Description != "Description of alpha" {
		t.Fatalf("after rescan = %+v", got)
	}
	off, _ := f.svc.ListSkills(ctx, domain.SkillFilter{State: domain.SkillDisabled})
	if len(off) != 1 || off[0].Name != "alpha" {
		t.Fatalf("disabled list = %+v", off)
	}

	// Wiping the DB and rescanning restores the disabled state from the sidecar.
	f.wipeDB()
	sum = f.scan()
	if sum.Disabled != 1 {
		t.Fatalf("scan after wipe = %+v", sum)
	}
	got, err = f.svc.GetSkill(ctx, alpha.ID)
	if err != nil || got.State != domain.SkillDisabled || got.Path != src {
		t.Fatalf("restored row = %+v err=%v", got, err)
	}

	enabled, err := f.svc.EnableSkill(ctx, alpha.ID)
	if err != nil {
		t.Fatal(err)
	}
	if enabled.State != domain.SkillEnabled || enabled.QuarantinePath != "" {
		t.Fatalf("enabled = %+v", enabled)
	}
	if !exists(filepath.Join(src, "SKILL.md")) || exists(wantQ) || exists(wantQ+".origin.json") {
		t.Fatal("skill not restored or quarantine not cleaned")
	}
	if again, err := f.svc.EnableSkill(ctx, alpha.ID); err != nil || again.State != domain.SkillEnabled {
		t.Fatalf("second enable: %+v %v", again, err)
	}
	sum = f.scan()
	if sum.Found != 2 || sum.Disabled != 0 || sum.Added != 0 {
		t.Fatalf("final scan = %+v", sum)
	}
}

func TestDisableSymlinkMovesLinkNotTarget(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	target := f.writeSkill("vault", "linked", "")
	link := f.path(".config/opencode/skills/linked")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	f.scan()
	sk := f.resolve("linked", "opencode")
	if !sk.IsSymlink || sk.LinkTarget != mustEval(t, target) {
		t.Fatalf("skill = %+v", sk)
	}
	if _, err := f.svc.DisableSkill(ctx, sk.ID); err != nil {
		t.Fatal(err)
	}
	if exists(link) {
		t.Fatal("link still in agent dir")
	}
	if !exists(filepath.Join(target, "SKILL.md")) {
		t.Fatal("symlink target was moved")
	}
	q := filepath.Join(f.dataDir, "disabled", "opencode", "linked")
	if info, err := os.Lstat(q); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("quarantined entry is not a link: %v", err)
	}
	if _, err := f.svc.EnableSkill(ctx, sk.ID); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("link not restored: %v", err)
	}
}

func TestDisableReadOnlyAndUnknown(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.writeSkill(".claude/plugins/cache/m/p/1.0.0/skills", "eas", "")
	f.scan()
	sk := f.resolve("eas", "claude")
	if _, err := f.svc.DisableSkill(ctx, sk.ID); !errors.Is(err, domain.ErrReadOnly) {
		t.Fatalf("err = %v", err)
	}
	if exists(filepath.Join(f.dataDir, "disabled")) {
		t.Fatal("quarantine dir created for a read-only skill")
	}
	if _, err := f.svc.DisableSkill(ctx, "nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown id err = %v", err)
	}
}

func TestResolveSkillAmbiguous(t *testing.T) {
	f := newFixture(t)
	f.writeSkill(".cursor/skills", "dup", "one\n")
	f.writeSkill(".agents/skills", "dup", "two\n")
	f.scan()
	_, err := f.svc.ResolveSkill(context.Background(), "dup", "cursor", "")
	var amb *AmbiguousError
	if !errors.As(err, &amb) || len(amb.Candidates) != 2 || !errors.Is(err, domain.ErrAmbiguous) {
		t.Fatalf("err = %v", err)
	}
}

func TestToggleAgentSkipsDisabledAgent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.writeSkill(".claude/skills", "alpha", "")
	f.writeSkill(".config/opencode/skills", "gamma", "")
	if _, err := f.svc.ToggleAgent(ctx, "opencode", false); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ToggleAgent(ctx, "nope", false); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown agent err = %v", err)
	}
	sum := f.scan()
	if sum.Found != 1 {
		t.Fatalf("scan = %+v", sum)
	}
	list, _ := f.svc.ListAgents(ctx)
	for _, a := range list {
		if a.ID == "opencode" && a.Enabled {
			t.Fatal("opencode should be disabled")
		}
		if a.ID == "claude-code" && !a.Enabled {
			t.Fatal("claude-code should stay enabled")
		}
	}
	if _, err := f.svc.ToggleAgent(ctx, "opencode", true); err != nil {
		t.Fatal(err)
	}
	if sum := f.scan(); sum.Found != 2 {
		t.Fatalf("scan after re-enable = %+v", sum)
	}
}

func TestProjectsRegisterScanRemove(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	root := f.path("work/proj")
	if err := os.MkdirAll(filepath.Join(root, ".agents", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.writeSkill("work/proj/.agents/skills", "shared", "")
	// Claude Code convention on this machine: .claude/skills/<name> -> ../../.agents/skills/<name>.
	if err := os.MkdirAll(filepath.Join(root, ".claude", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../.agents/skills/shared", filepath.Join(root, ".claude", "skills", "shared")); err != nil {
		t.Fatal(err)
	}

	empty := f.path("work/empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.svc.RegisterProject(ctx, empty); err == nil {
		t.Fatal("registering a dir without .git or skills dirs should fail")
	}
	if _, _, err := f.svc.RegisterProject(ctx, f.path("missing")); err == nil {
		t.Fatal("registering a missing dir should fail")
	}

	p, sum, err := f.svc.RegisterProject(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "proj" || p.Root != root {
		t.Fatalf("project = %+v", p)
	}
	// claude-code, codex, cursor(x2: .cursor no, .agents yes, .claude yes), gemini-cli, antigravity, opencode(x2).
	if sum.Found != 8 {
		t.Fatalf("project scan = %+v", sum)
	}
	if _, _, err := f.svc.RegisterProject(ctx, root); !errors.Is(err, domain.ErrExists) {
		t.Fatalf("duplicate register err = %v", err)
	}
	sk, err := f.svc.ResolveSkill(ctx, "shared", "claude-code", root)
	if err != nil || !sk.IsSymlink || sk.ProjectID == nil || *sk.ProjectID != p.ID {
		t.Fatalf("project skill = %+v err=%v", sk, err)
	}
	if got, _ := f.svc.ListSkills(ctx, domain.SkillFilter{Scope: domain.ProjectScope(root)}); len(got) != 8 {
		t.Fatalf("project list = %d", len(got))
	}
	if got, _ := f.svc.ListSkills(ctx, domain.SkillFilter{Scope: domain.GlobalScope}); len(got) != 0 {
		t.Fatalf("global list = %d", len(got))
	}

	// Disabling a project skill uses a project-specific quarantine dir.
	disabled, err := f.svc.DisableSkill(ctx, sk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(filepath.Dir(disabled.QuarantinePath)) != filepath.Join(f.dataDir, "disabled", "claude-code", "projects") {
		t.Fatalf("quarantine path = %s", disabled.QuarantinePath)
	}
	if _, err := f.svc.EnableSkill(ctx, sk.ID); err != nil {
		t.Fatal(err)
	}

	// Full scan includes the project.
	if sum := f.scan(); sum.Found != 8 {
		t.Fatalf("full scan = %+v", sum)
	}
	removed, err := f.svc.RemoveProject(ctx, root)
	if err != nil || removed.ID != p.ID {
		t.Fatalf("RemoveProject = %+v err=%v", removed, err)
	}
	if got, _ := f.svc.ListSkills(ctx, domain.SkillFilter{}); len(got) != 0 {
		t.Fatalf("skills after project removal = %d", len(got))
	}
	if list, _ := f.svc.ListProjects(ctx); len(list) != 0 {
		t.Fatalf("projects = %+v", list)
	}
	if _, err := f.svc.RemoveProject(ctx, root); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("remove twice err = %v", err)
	}
}

func TestDoctorReportsIssues(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.writeSkill(".claude/skills", "same", "v1\n")
	f.writeSkill(".agents/skills", "same", "v2\n")
	f.writeRaw(".claude/skills", "bad-fm", "---\nname: Wrong Name\n---\nbody\n")
	f.writeRaw(".claude/skills", "no-fm", "# plain\n")
	if err := os.Symlink(f.path("nowhere"), f.path(".claude/skills/dangling")); err != nil {
		t.Fatal(err)
	}
	f.writeSkill(".claude/skills", "conflict", "")
	f.scan()
	conflict := f.resolve("conflict", "claude-code")
	if _, err := f.svc.DisableSkill(ctx, conflict.ID); err != nil {
		t.Fatal(err)
	}
	f.writeSkill(".claude/skills", "conflict", "recreated\n")

	report, err := f.svc.Doctor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{IssueInvalidFrontmatter: 2, IssueDanglingSymlink: 1, IssueDrift: 1, IssueQuarantineConflict: 1}
	for kind, n := range want {
		if report.Counts[kind] != n {
			t.Errorf("%s count = %d, want %d (issues: %+v)", kind, report.Counts[kind], n, report.Issues)
		}
	}
	for _, i := range report.Issues {
		if i.Kind == IssueDrift {
			if i.Name != "same" || len(i.SkillIDs) != 4 || len(i.Details) != 4 {
				t.Errorf("drift issue = %+v", i)
			}
		}
		if i.Kind == IssueDanglingSymlink && i.Name != "dangling" {
			t.Errorf("dangling issue = %+v", i)
		}
		if i.Kind == IssueQuarantineConflict && i.SkillID != conflict.ID {
			t.Errorf("conflict issue = %+v", i)
		}
	}
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
