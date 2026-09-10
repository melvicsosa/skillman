package skillssh

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/melvicsosa/skillman/internal/domain"
)

// fakeSite replays the recorded fixtures: v1 endpoints need a bearer token,
// the legacy search is public (as observed on skills.sh).
func fakeSite(t *testing.T) *httptest.Server {
	t.Helper()
	serve := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer tok" {
				w.WriteHeader(401)
				http.ServeFile(w, r, filepath.Join("testdata", "v1_unauthorized.json"))
				return
			}
			http.ServeFile(w, r, filepath.Join("testdata", name))
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "react" {
			http.Error(w, "unexpected q", 400)
			return
		}
		http.ServeFile(w, r, filepath.Join("testdata", "legacy_search.json"))
	})
	mux.HandleFunc("GET /api/v1/skills/search", serve("v1_search.json"))
	mux.HandleFunc("GET /api/v1/skills", serve("v1_trending.json"))
	mux.HandleFunc("GET /api/v1/skills/curated", serve("v1_curated.json"))
	mux.HandleFunc("GET /api/v1/skills/vercel-labs/skills/find-skills", serve("v1_detail.json"))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

type fakeRepos struct {
	dir  string
	refs []string
}

func (f *fakeRepos) Fetch(_ context.Context, ref string) (domain.FetchResult, error) {
	f.refs = append(f.refs, ref)
	return domain.FetchResult{Dir: f.dir, Source: domain.VaultSource{Type: domain.SourceGitHub, Commit: "c0ffee"}, Cleanup: func() {}}, nil
}

func TestParseRef(t *testing.T) {
	src, slug, ok := ParseRef("skillssh:vercel-labs/skills/find-skills")
	if !ok || src != "vercel-labs/skills" || slug != "find-skills" {
		t.Fatalf("ParseRef = %q %q %v", src, slug, ok)
	}
	for _, bad := range []string{"vercel-labs/skills/find-skills", "skillssh:", "skillssh:only", "skillssh:a/"} {
		if _, _, ok := ParseRef(bad); ok {
			t.Errorf("ParseRef(%q) accepted", bad)
		}
	}
}

func TestLegacySearchWithoutToken(t *testing.T) {
	srv := fakeSite(t)
	c := New("", nil)
	c.BaseURL = srv.URL
	hits, err := c.Search(context.Background(), "react")
	if err != nil || len(hits) != 5 {
		t.Fatalf("hits = %d err=%v", len(hits), err)
	}
	h := hits[0]
	if h.ID != "vercel-labs/agent-skills/vercel-react-best-practices" || h.Ref != "skillssh:"+h.ID || h.Installs != 702718 || h.Source != "vercel-labs/agent-skills" || h.Registry != ID {
		t.Fatalf("hit = %+v", h)
	}
	if _, err := c.Search(context.Background(), "r"); err == nil {
		t.Error("short query accepted")
	}
}

func TestV1EndpointsWithToken(t *testing.T) {
	srv := fakeSite(t)
	c := New("tok", nil)
	c.BaseURL = srv.URL
	ctx := context.Background()
	hits, err := c.Search(ctx, "react native")
	if err != nil || len(hits) != 1 || hits[0].Name != "React Native" || hits[0].Ref != "skillssh:expo/skills/react-native" {
		t.Fatalf("v1 search = %+v err=%v", hits, err)
	}
	trending, err := c.Trending(ctx)
	if err != nil || len(trending) != 1 || trending[0].ID != "vercel-labs/skills/find-skills" {
		t.Fatalf("trending = %+v err=%v", trending, err)
	}
	curated, err := c.Curated(ctx)
	if err != nil || len(curated) != 1 || curated[0].Owner != "vercel-labs" || len(curated[0].Skills) != 1 {
		t.Fatalf("curated = %+v err=%v", curated, err)
	}
	res, err := c.Fetch(ctx, "skillssh:vercel-labs/skills/find-skills")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Cleanup()
	if res.Name != "find-skills" || res.Source.Type != domain.SourceSkillSH || res.Source.Commit != "a1b2c3d4e5f6" {
		t.Fatalf("fetch = %+v", res)
	}
	if b, err := os.ReadFile(filepath.Join(res.Dir, "examples", "app-router.ts")); err != nil || string(b) != "// Example code" {
		t.Fatalf("file: %v", err)
	}
}

func TestV1WithoutTokenIsAuthRequired(t *testing.T) {
	srv := fakeSite(t)
	c := New("", nil)
	c.BaseURL = srv.URL
	if _, err := c.Trending(context.Background()); !errors.Is(err, domain.ErrAuthRequired) {
		t.Fatalf("trending err = %v", err)
	}
	if _, err := c.Curated(context.Background()); !errors.Is(err, domain.ErrAuthRequired) {
		t.Fatalf("curated err = %v", err)
	}
	if _, err := c.Fetch(context.Background(), "skillssh:vercel-labs/skills/find-skills"); !errors.Is(err, domain.ErrAuthRequired) {
		t.Fatalf("fetch without repos err = %v", err)
	}
}

func TestFetchFallsBackToGitHubRepo(t *testing.T) {
	repo := t.TempDir()
	for _, p := range []string{"skills/find-skills", "other/deep/find-skills"} {
		os.MkdirAll(filepath.Join(repo, p), 0o755)
		os.WriteFile(filepath.Join(repo, p, "SKILL.md"), []byte("---\nname: find-skills\ndescription: d\n---\n"), 0o644)
	}
	repos := &fakeRepos{dir: repo}
	c := New("", repos)
	res, err := c.Fetch(context.Background(), "skillssh:vercel-labs/skills/find-skills")
	if err != nil {
		t.Fatal(err)
	}
	if repos.refs[0] != "vercel-labs/skills" || res.Dir != filepath.Join(repo, "skills", "find-skills") || res.Source.Subpath != "skills/find-skills" || res.Source.Commit != "c0ffee" || res.Source.Type != domain.SourceSkillSH {
		t.Fatalf("fetch = %+v refs=%v", res, repos.refs)
	}
	if _, err := c.Fetch(context.Background(), "skillssh:vercel-labs/skills/missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing slug err = %v", err)
	}
}

func TestFindSkillDirByDeclaredName(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "skills", "react-best-practices")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: vercel-react-best-practices\ndescription: React guidance\n---\nBody\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, rel, err := findSkillDir(root, "vercel-react-best-practices")
	if err != nil {
		t.Fatalf("findSkillDir: %v", err)
	}
	if got != dir || rel != "skills/react-best-practices" {
		t.Fatalf("got %q %q, want %q", got, rel, dir)
	}
}
