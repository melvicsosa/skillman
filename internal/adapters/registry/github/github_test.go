package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/melvicsosa/skillman/internal/adapters/registry/archive/archivetest"
	"github.com/melvicsosa/skillman/internal/domain"
)

func TestParseRef(t *testing.T) {
	tests := []struct {
		in   string
		want Ref
		ok   bool
	}{
		{"anthropics/skills", Ref{Owner: "anthropics", Repo: "skills"}, true},
		{"anthropics/skills/skills/frontend-design", Ref{Owner: "anthropics", Repo: "skills", Subpath: "skills/frontend-design"}, true},
		{"o/r/sub@v1.2", Ref{Owner: "o", Repo: "r", Subpath: "sub", Ref: "v1.2"}, true},
		{"https://github.com/o/r", Ref{Owner: "o", Repo: "r"}, true},
		{"https://github.com/o/r.git", Ref{Owner: "o", Repo: "r"}, true},
		{"https://github.com/o/r/tree/dev/skills/x", Ref{Owner: "o", Repo: "r", Ref: "dev", Subpath: "skills/x"}, true},
		{"https://github.com/o/r/blob/main/skills/x/SKILL.md", Ref{Owner: "o", Repo: "r", Ref: "main", Subpath: "skills/x"}, true},
		{"github.com/o/r", Ref{Owner: "o", Repo: "r"}, true},
		{"github:o/r", Ref{Owner: "o", Repo: "r"}, true},
		{"./local/path", Ref{}, false},
		{"/abs/path", Ref{}, false},
		{"single", Ref{}, false},
		{"https://example.com/o/r", Ref{}, false},
		{"skillssh:o/r/x", Ref{}, false},
	}
	for _, tt := range tests {
		got, ok := ParseRef(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseRef(%q) = %+v, %v; want %+v, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func fakeAPI(t *testing.T) (*httptest.Server, *string) {
	t.Helper()
	tarball := archivetest.TarGz(t, map[string]string{
		"o-r-abc1234/README.md":                "readme",
		"o-r-abc1234/skills/alpha/SKILL.md":    "---\nname: alpha\ndescription: A\n---\nbody\n",
		"o-r-abc1234/skills/alpha/ref/more.md": "more",
		"o-r-abc1234/skills/beta/SKILL.md":     "---\nname: beta\ndescription: B\n---\n",
	})
	var lastAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/o/r", func(w http.ResponseWriter, r *http.Request) {
		lastAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"default_branch":"develop"}`))
	})
	mux.HandleFunc("GET /repos/o/r/tarball/{ref}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("ref") != "develop" && r.PathValue("ref") != "v1" {
			http.Error(w, `{"message":"Not Found"}`, 404)
			return
		}
		w.Write(tarball)
	})
	mux.HandleFunc("GET /repos/o/missing", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, 404)
	})
	mux.HandleFunc("GET /search/repositories", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") == "" {
			http.Error(w, "no q", 400)
			return
		}
		w.Write([]byte(`{"items":[{"full_name":"o/r","description":"d","html_url":"https://github.com/o/r","stargazers_count":5}]}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &lastAuth
}

func TestFetchSubpathDefaultBranchAndToken(t *testing.T) {
	srv, auth := fakeAPI(t)
	c := New("tok")
	c.BaseURL = srv.URL
	res, err := c.Fetch(context.Background(), "o/r/skills/alpha")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Cleanup()
	if *auth != "Bearer tok" {
		t.Errorf("auth header = %q", *auth)
	}
	if res.Name != "alpha" || res.Source.Type != domain.SourceGitHub || res.Source.Commit != "abc1234" || res.Source.Subpath != "skills/alpha" || res.Source.Ref != "o/r/skills/alpha" {
		t.Fatalf("result = %+v", res)
	}
	if res.Source.URL != "https://github.com/o/r/tree/develop/skills/alpha" {
		t.Errorf("url = %s", res.Source.URL)
	}
	if b, err := os.ReadFile(filepath.Join(res.Dir, "ref", "more.md")); err != nil || string(b) != "more" {
		t.Fatalf("nested file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(res.Dir, "README.md")); err == nil {
		t.Fatal("root file leaked into subpath extraction")
	}
	res.Cleanup()
	if _, err := os.Stat(res.Dir); err == nil {
		t.Fatal("cleanup left the temp dir")
	}
}

func TestFetchWholeRepoExplicitRefAndErrors(t *testing.T) {
	srv, _ := fakeAPI(t)
	c := New("")
	c.BaseURL = srv.URL
	res, err := c.Fetch(context.Background(), "https://github.com/o/r/tree/v1")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Cleanup()
	if res.Name != "" || res.Source.Subpath != "" {
		t.Fatalf("result = %+v", res)
	}
	if _, err := os.Stat(filepath.Join(res.Dir, "skills", "beta", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Fetch(context.Background(), "o/r/skills/nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("missing subpath err = %v", err)
	}
	if _, err := c.Fetch(context.Background(), "o/missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("missing repo err = %v", err)
	}
	if _, err := c.Fetch(context.Background(), "./nope"); err == nil {
		t.Error("non-github ref accepted")
	}
}

func TestSearch(t *testing.T) {
	srv, _ := fakeAPI(t)
	c := New("")
	c.BaseURL = srv.URL
	hits, err := c.Search(context.Background(), "skills")
	if err != nil || len(hits) != 1 || hits[0].Ref != "o/r" || hits[0].Registry != ID || hits[0].Installs != 5 {
		t.Fatalf("hits = %+v err=%v", hits, err)
	}
}
