package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/domain"
)

// fakeRegistry serves refs from local directories registered in sources
// (ref -> dir). Fetch copies the dir so the caller's cleanup is harmless.
type fakeRegistry struct {
	id      string
	sources map[string]string
	hits    []domain.RegistryResult
	fetched int
}

func (f *fakeRegistry) ID() string { return f.id }
func (f *fakeRegistry) Matches(ref string) bool {
	return strings.HasPrefix(ref, f.id+":")
}
func (f *fakeRegistry) Search(_ context.Context, q string) ([]domain.RegistryResult, error) {
	if q == "boom" {
		return nil, errors.New("registry exploded")
	}
	return f.hits, nil
}
func (f *fakeRegistry) Trending(context.Context) ([]domain.RegistryResult, error) { return f.hits, nil }
func (f *fakeRegistry) Fetch(_ context.Context, ref string) (domain.FetchResult, error) {
	src, ok := f.sources[ref]
	if !ok {
		return domain.FetchResult{}, fmt.Errorf("%s: %w", ref, domain.ErrNotFound)
	}
	f.fetched++
	tmp, err := os.MkdirTemp("", "fake-*")
	if err != nil {
		return domain.FetchResult{}, err
	}
	dir := filepath.Join(tmp, filepath.Base(src))
	if err := skillfs.CopyTree(src, dir); err != nil {
		return domain.FetchResult{}, err
	}
	return domain.FetchResult{Dir: dir, Source: domain.VaultSource{Type: f.id, Ref: ref, Commit: "c1"}, Cleanup: func() { os.RemoveAll(tmp) }}, nil
}

// source writes a skill source outside any agent dir and registers it as ref.
func (f *fixture) source(ref, name, body string) string {
	dir := f.writeSkill("sources", name, body)
	f.reg.sources[ref] = dir
	return dir
}

func (f *fixture) add(ref string, opts InstallOptions) AddResult {
	f.t.Helper()
	res, err := f.svc.Add(context.Background(), ref, opts)
	if err != nil {
		f.t.Fatalf("Add(%s): %v", ref, err)
	}
	return res
}

func readLink(t *testing.T, p string) string {
	t.Helper()
	target, err := os.Readlink(p)
	if err != nil {
		t.Fatalf("readlink %s: %v", p, err)
	}
	return target
}

func TestAddStoresInVaultAndLinksGlobally(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.source("fake:alpha", "alpha", "body A\n")

	res := f.add("fake:alpha", InstallOptions{Agents: []domain.AgentID{"codex", "claude-code"}})
	if res.Shape != domain.ShapeSkill || len(res.Entries) != 1 || len(res.Links) != 2 {
		t.Fatalf("add = %+v", res)
	}
	entry := res.Entries[0]
	vaultDir := filepath.Join(f.dataDir, "vault", "alpha")
	if entry.Path != vaultDir || entry.Source.Ref != "fake:alpha" || entry.Source.Commit != "c1" || !entry.Spec.Valid || entry.SourceHash == "" {
		t.Fatalf("entry = %+v", entry)
	}
	var sidecar domain.VaultEntry
	b, err := os.ReadFile(vaultDir + ".vault.json")
	if err != nil || json.Unmarshal(b, &sidecar) != nil || sidecar.Name != "alpha" || sidecar.Source.Ref != "fake:alpha" {
		t.Fatalf("sidecar: %v %s", err, b)
	}
	codexLink := f.path(".agents/skills/alpha")
	if readLink(t, codexLink) != vaultDir || readLink(t, f.path(".claude/skills/alpha")) != vaultDir {
		t.Fatal("global links do not point at the vault")
	}

	sk := f.resolve("alpha", "codex")
	if sk.VaultRef != "alpha" || !sk.IsSymlink {
		t.Fatalf("codex skill = %+v", sk)
	}
	// cursor and gemini share ~/.agents/skills, so they see it too.
	if sk := f.resolve("alpha", "cursor"); sk.VaultRef != "alpha" {
		t.Fatalf("cursor skill = %+v", sk)
	}
	list, err := f.svc.VaultList(ctx)
	if err != nil || len(list) != 1 || len(list[0].Links) != 4 {
		t.Fatalf("VaultList = %+v err=%v", list, err)
	}

	// Re-installing is idempotent.
	links, err := f.svc.InstallFromVault(ctx, "alpha", InstallOptions{Agents: []domain.AgentID{"codex"}})
	if err != nil || len(links) != 1 || !links[0].Existing {
		t.Fatalf("reinstall = %+v err=%v", links, err)
	}
	// Existing foreign dir is refused.
	f.writeSkill(".config/opencode/skills", "alpha", "someone else\n")
	if _, err := f.svc.InstallFromVault(ctx, "alpha", InstallOptions{Agents: []domain.AgentID{"opencode"}}); !errors.Is(err, domain.ErrExists) {
		t.Fatalf("install over foreign dir err = %v", err)
	}
	// Plugins agent is read-only.
	if _, err := f.svc.InstallFromVault(ctx, "alpha", InstallOptions{Agents: []domain.AgentID{"claude"}}); !errors.Is(err, domain.ErrReadOnly) {
		t.Fatalf("install into plugins err = %v", err)
	}

	// Remove refuses while linked, then unlinks with force.
	if _, err := f.svc.VaultRemove(ctx, "alpha", false); !errors.Is(err, domain.ErrLinked) {
		t.Fatalf("remove linked err = %v", err)
	}
	rm, err := f.svc.VaultRemove(ctx, "alpha", true)
	if err != nil || len(rm.Unlinked) != 2 {
		t.Fatalf("force remove = %+v err=%v", rm, err)
	}
	if exists(codexLink) || exists(vaultDir) || exists(vaultDir+".vault.json") {
		t.Fatal("force remove left files behind")
	}
	if !exists(f.path(".config/opencode/skills/alpha")) {
		t.Fatal("foreign dir was removed")
	}
	if _, err := f.svc.VaultGet(ctx, "alpha"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("after remove err = %v", err)
	}
}

func TestAddProjectCopyUninstallAndCommandStub(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.source("fake:beta", "beta", "body B\n")
	project := filepath.Join(t.TempDir(), "proj")
	os.MkdirAll(filepath.Join(project, ".git"), 0o755)

	res := f.add("fake:beta", InstallOptions{Agents: []domain.AgentID{"claude-code", "codex"}, Root: project, Copy: true, Command: true})
	if len(res.Links) != 2 {
		t.Fatalf("links = %+v", res.Links)
	}
	shared := filepath.Join(project, ".agents", "skills", "beta")
	claude := filepath.Join(project, ".claude", "skills", "beta")
	if info, err := os.Lstat(shared); err != nil || !info.IsDir() {
		t.Fatalf("shared copy: %v", err)
	}
	if readLink(t, claude) != filepath.Join("..", "..", ".agents", "skills", "beta") {
		t.Fatalf("claude link = %s", readLink(t, claude))
	}
	stub, err := os.ReadFile(filepath.Join(project, ".claude", "commands", "beta.md"))
	if err != nil || !strings.Contains(string(stub), "Description of beta") {
		t.Fatalf("command stub: %v %s", err, stub)
	}
	projects, _ := f.svc.ListProjects(ctx)
	if len(projects) != 1 || projects[0].Root != project {
		t.Fatalf("project not registered: %+v", projects)
	}
	sk, err := f.svc.ResolveSkill(ctx, "beta", "claude-code", project)
	if err != nil || sk.VaultRef != "beta" || !sk.IsSymlink {
		t.Fatalf("claude project skill = %+v err=%v", sk, err)
	}
	sk, err = f.svc.ResolveSkill(ctx, "beta", "codex", project)
	if err != nil || sk.VaultRef != "beta" || sk.IsSymlink {
		t.Fatalf("codex project copy = %+v err=%v", sk, err)
	}

	removed, err := f.svc.UninstallFromVault(ctx, "beta", "claude-code", project)
	if err != nil || len(removed) != 1 || removed[0] != claude || exists(claude) || !exists(shared) {
		t.Fatalf("uninstall claude = %v err=%v", removed, err)
	}
	if _, err := f.svc.UninstallFromVault(ctx, "beta", "claude-code", project); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("second uninstall err = %v", err)
	}
	removed, err = f.svc.UninstallFromVault(ctx, "beta", "codex", project)
	if err != nil || len(removed) != 1 || exists(shared) {
		t.Fatalf("uninstall codex = %v err=%v", removed, err)
	}
	if list, _ := f.svc.VaultList(ctx); len(list[0].Links) != 0 {
		t.Fatalf("links after uninstall = %+v", list[0].Links)
	}
}

func TestVaultUpdateReplacesWhenSourceChanged(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	src := f.source("fake:gamma", "gamma", "v1\n")
	f.add("fake:gamma", InstallOptions{Agents: []domain.AgentID{"codex"}, Copy: true})
	f.add("fake:gamma", InstallOptions{Agents: []domain.AgentID{"opencode"}})

	res, err := f.svc.VaultUpdate(ctx, "gamma")
	if err != nil || res.Updated {
		t.Fatalf("no-op update = %+v err=%v", res, err)
	}
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("---\nname: gamma\ndescription: Description of gamma\n---\nv2\n"), 0o644)
	res, err = f.svc.VaultUpdate(ctx, "gamma")
	if err != nil || !res.Updated || res.OldSourceHash == res.NewSourceHash || len(res.CopiesRefresh) != 1 {
		t.Fatalf("update = %+v err=%v", res, err)
	}
	for _, p := range []string{filepath.Join(f.dataDir, "vault", "gamma", "SKILL.md"), f.path(".agents/skills/gamma/SKILL.md"), f.path(".config/opencode/skills/gamma/SKILL.md")} {
		b, err := os.ReadFile(p)
		if err != nil || !strings.HasSuffix(string(b), "v2\n") {
			t.Errorf("%s not updated: %v %q", p, err, b)
		}
	}
	entry, _ := f.svc.VaultGet(ctx, "gamma")
	if entry.UpdatedAt.Before(entry.InstalledAt) || entry.ContentHash == res.OldSourceHash || len(entry.Links) != 4 {
		t.Fatalf("entry after update = %+v", entry)
	}
	if sk := f.resolve("gamma", "codex"); sk.VaultRef != "gamma" {
		t.Fatalf("copy lost its vault ref: %+v", sk)
	}
}

func TestAddRejectedIsReportedByDoctor(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	bad := f.path("sources/bad")
	os.MkdirAll(bad, 0o755)
	os.WriteFile(filepath.Join(bad, "README.md"), []byte("nope"), 0o644)
	f.reg.sources["fake:bad"] = bad
	_, err := f.svc.Add(ctx, "fake:bad", InstallOptions{})
	if !errors.Is(err, domain.ErrRejected) {
		t.Fatalf("Add(bad) err = %v", err)
	}
	report, err := f.svc.Doctor(ctx)
	if err != nil || report.Counts[IssueRejectedConversion] != 1 {
		t.Fatalf("doctor = %+v err=%v", report.Counts, err)
	}
	if err := f.svc.ClearRejections(); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Add(ctx, "nope:x", InstallOptions{}); err == nil {
		t.Fatal("unknown ref form accepted")
	}
	if _, err := f.svc.Add(ctx, "fake:missing", InstallOptions{}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing ref err = %v", err)
	}
}

func TestAddConflictingNameAndVaultRebuildAfterWipe(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.source("fake:delta", "delta", "d\n")
	other := f.writeSkill("elsewhere", "delta", "other\n")
	f.reg.sources["fake:delta2"] = other
	f.add("fake:delta", InstallOptions{Agents: []domain.AgentID{"codex"}})
	if _, err := f.svc.Add(ctx, "fake:delta2", InstallOptions{}); !errors.Is(err, domain.ErrExists) {
		t.Fatalf("conflicting add err = %v", err)
	}
	// Same ref again is a refresh, not a conflict.
	f.add("fake:delta", InstallOptions{})

	f.wipeDB()
	f.scan()
	list, err := f.svc.VaultList(ctx)
	if err != nil || len(list) != 1 || list[0].Source.Ref != "fake:delta" || len(list[0].Links) != 3 {
		t.Fatalf("after wipe = %+v err=%v", list, err)
	}
	// A vault dir removed by hand is dropped from the cache and its links reported.
	os.RemoveAll(filepath.Join(f.dataDir, "vault", "delta"))
	sum := f.scan()
	if list, _ := f.svc.VaultList(ctx); len(list) != 0 || len(sum.Errors) == 0 {
		t.Fatalf("dangling vault: list=%+v errors=%v", list, sum.Errors)
	}
	report, _ := f.svc.Doctor(ctx)
	if report.Counts[IssueVaultBrokenLink] != 3 {
		t.Fatalf("doctor counts = %+v", report.Counts)
	}
}

func TestSearchMergesRegistriesAndReportsFailures(t *testing.T) {
	f := newFixture(t)
	f.reg.hits = []domain.RegistryResult{{Registry: "fake", ID: "a", Name: "a", Ref: "fake:a"}}
	res, err := f.svc.Search(context.Background(), "x", "")
	if err != nil || len(res.Results) != 1 {
		t.Fatalf("search = %+v err=%v", res, err)
	}
	if _, err := f.svc.Search(context.Background(), "boom", ""); err == nil {
		t.Fatal("all-failed search returned no error")
	}
	if _, err := f.svc.Search(context.Background(), "x", "nope"); err == nil {
		t.Fatal("unknown source accepted")
	}
	if hits, err := f.svc.Trending(context.Background()); err != nil || len(hits) != 1 {
		t.Fatalf("trending = %+v err=%v", hits, err)
	}
	if _, err := f.svc.Curated(context.Background()); !errors.Is(err, domain.ErrUnsupported) {
		t.Fatalf("curated err = %v", err)
	}
}

func TestImportLock(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.writeSkill(".agents/skills", "find-skills", "find\n")
	f.writeSkill(".agents/skills", "already", "a\n")
	f.reg.sources["fake:already"] = f.writeSkill("sources", "already", "a\n")
	f.add("fake:already", InstallOptions{})
	lock := `{"version":3,"skills":{
	  "find-skills":{"source":"vercel-labs/skills","sourceType":"github","sourceUrl":"https://github.com/vercel-labs/skills.git","skillPath":"skills/find-skills/SKILL.md","skillFolderHash":"3013fdeb","installedAt":"2026-06-06T18:46:48.537Z","updatedAt":"2026-06-06T18:46:48.537Z"},
	  "already":{"source":"o/r","sourceType":"github","sourceUrl":"https://github.com/o/r.git","skillPath":"already/SKILL.md","skillFolderHash":"x"},
	  "gone":{"source":"o/r","sourceType":"github","sourceUrl":"https://github.com/o/r.git","skillPath":"skills/gone/SKILL.md","skillFolderHash":"y"}}}`
	if err := os.WriteFile(f.path(".agents/.skill-lock.json"), []byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := f.svc.LockPath(); got != f.path(".agents/.skill-lock.json") {
		t.Fatalf("LockPath = %s", got)
	}
	res, err := f.svc.ImportLock(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Imported) != 1 || res.Imported[0] != "find-skills" || len(res.Skipped) != 2 {
		t.Fatalf("import = %+v", res)
	}
	entry, err := f.svc.VaultGet(ctx, "find-skills")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Source.Type != domain.SourceLock || entry.Source.Ref != "vercel-labs/skills/skills/find-skills" || entry.Source.Subpath != "skills/find-skills" || entry.Source.Commit != "3013fdeb" || entry.InstalledAt.Year() != 2026 {
		t.Fatalf("entry = %+v", entry)
	}
	// The installed copy is attributed to the vault entry by hash.
	if len(entry.Links) != 3 || entry.Links[0].IsSymlink {
		t.Fatalf("links = %+v", entry.Links)
	}
	if !exists(f.path(".agents/skills/find-skills/SKILL.md")) {
		t.Fatal("installed dir was moved")
	}
}
