package marketplace

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/melvicsosa/skillman/internal/domain"
)

type fakeGitHub struct {
	files   map[string][]byte // "owner/repo" -> manifest
	fetched []string
	loads   int
}

func (f *fakeGitHub) RawFile(_ context.Context, owner, repo, _, filePath string) ([]byte, error) {
	f.loads++
	if filePath != ManifestPath {
		return nil, errors.New("unexpected path " + filePath)
	}
	b, ok := f.files[owner+"/"+repo]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return b, nil
}

func (f *fakeGitHub) Fetch(_ context.Context, ref string) (domain.FetchResult, error) {
	f.fetched = append(f.fetched, ref)
	return domain.FetchResult{Dir: "/tmp/x", Source: domain.VaultSource{Type: domain.SourceGitHub, Ref: ref, URL: "https://github.com/x", Commit: "abc"}, Cleanup: func() {}}, nil
}

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/marketplace.json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func newClient(t *testing.T) (*Client, *fakeGitHub) {
	gh := &fakeGitHub{files: map[string][]byte{"anthropics/claude-plugins-official": fixture(t)}}
	c := New(gh, func(context.Context) []string { return []string{"anthropics/claude-plugins-official", "nobody/none"} })
	return c, gh
}

func TestParseRef(t *testing.T) {
	cases := map[string]Ref{
		"marketplace:anthropics/claude-plugins-official/asana": {Owner: "anthropics", Repo: "claude-plugins-official", Plugin: "asana"},
		"marketplace:o/r":  {Owner: "o", Repo: "r"},
		"marketplace:o/r/": {Owner: "o", Repo: "r"},
	}
	for in, want := range cases {
		got, ok := ParseRef(in)
		if !ok || got != want {
			t.Errorf("ParseRef(%q) = %+v %v", in, got, ok)
		}
	}
	for _, bad := range []string{"o/r/p", "marketplace:o", "marketplace:o/r/p/q", "github:o/r", "marketplace:o/r/sp ace"} {
		if _, ok := ParseRef(bad); ok {
			t.Errorf("ParseRef(%q) accepted", bad)
		}
	}
}

func TestParseRealShape(t *testing.T) {
	m, err := Parse(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "claude-plugins-official" || m.Owner.Name != "Anthropic" || len(m.Plugins) != 3 {
		t.Fatalf("manifest = %+v", m)
	}
	rel, ok := m.Find("agent-sdk-dev")
	if !ok || rel.Source.Path != "./plugins/agent-sdk-dev" || rel.Source.Kind != "" {
		t.Fatalf("relative plugin = %+v", rel)
	}
	sub, ok := m.Find("42crunch-api-security-testing")
	if !ok || sub.Source.Kind != "git-subdir" || sub.Source.Dir != "plugins/api-security-testing" || sub.Source.Ref != "v1.5.5" || sub.Source.Sha == "" {
		t.Fatalf("git-subdir plugin = %+v", sub)
	}
}

func TestGitHubRef(t *testing.T) {
	cases := []struct {
		p    Plugin
		want string
	}{
		{Plugin{Source: PluginSource{Path: "./plugins/foo"}}, "o/r/plugins/foo"},
		{Plugin{Source: PluginSource{Path: "."}}, "o/r"},
		{Plugin{Source: PluginSource{Kind: "url", URL: "https://github.com/a/b.git", Sha: "deadbeef"}}, "a/b@deadbeef"},
		{Plugin{Source: PluginSource{Kind: "git-subdir", URL: "https://github.com/a/b.git", Dir: "plugins/x", Ref: "v1", Sha: "s"}}, "a/b/plugins/x@v1"},
		{Plugin{Source: PluginSource{Kind: "github", Repo: "a/b", Dir: "p"}}, "a/b/p"},
	}
	for _, c := range cases {
		got, err := c.p.GitHubRef("o/r")
		if err != nil || got != c.want {
			t.Errorf("GitHubRef(%+v) = %q, %v; want %q", c.p.Source, got, err, c.want)
		}
	}
	if _, err := (Plugin{Source: PluginSource{Kind: "url", URL: "https://gitlab.com/a/b.git"}}).GitHubRef("o/r"); !errors.Is(err, domain.ErrUnsupported) {
		t.Errorf("non-github url err = %v", err)
	}
}

func TestFetchResolvesThroughGitHub(t *testing.T) {
	c, gh := newClient(t)
	res, err := c.Fetch(context.Background(), "marketplace:anthropics/claude-plugins-official/asana")
	if err != nil {
		t.Fatal(err)
	}
	if len(gh.fetched) != 1 || gh.fetched[0] != "anthropics/claude-plugins-official/external_plugins/asana" {
		t.Fatalf("fetched %v", gh.fetched)
	}
	if res.Name != "asana" || res.Source.Type != domain.SourceMarketplace || res.Source.Ref != "marketplace:anthropics/claude-plugins-official/asana" || res.Source.Commit != "abc" {
		t.Fatalf("result = %+v", res)
	}
	res, err = c.Fetch(context.Background(), "marketplace:anthropics/claude-plugins-official/42crunch-api-security-testing")
	if err != nil || gh.fetched[1] != "42Crunch-AI/claude-plugins/plugins/api-security-testing@v1.5.5" || res.Source.Commit != "30287f5e3f122a646d1ac5ca3ab96e130c52a3ad" {
		t.Fatalf("git-subdir fetch = %+v %v %v", res, err, gh.fetched)
	}
	if gh.loads != 1 {
		t.Fatalf("manifest loaded %d times, want cached", gh.loads)
	}
	if _, err := c.Fetch(context.Background(), "marketplace:anthropics/claude-plugins-official/nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown plugin err = %v", err)
	}
	if _, err := c.Fetch(context.Background(), "marketplace:anthropics/claude-plugins-official"); err == nil || !strings.Contains(err.Error(), "pick one") {
		t.Fatalf("whole marketplace err = %v", err)
	}
	if _, err := c.Fetch(context.Background(), "marketplace:nobody/none/x"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing manifest err = %v", err)
	}
}

func TestSearch(t *testing.T) {
	c, _ := newClient(t)
	hits, err := c.Search(context.Background(), "SDK")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Name != "agent-sdk-dev" || hits[0].Ref != "marketplace:anthropics/claude-plugins-official/agent-sdk-dev" || hits[0].Description == "" || hits[0].Registry != ID {
		t.Fatalf("hits = %+v", hits)
	}
	if hits, err = c.Search(context.Background(), "agent SDK"); err != nil || len(hits) != 1 || hits[0].Name != "agent-sdk-dev" {
		t.Fatalf("multi-term hits = %+v %v", hits, err)
	}
	hits, err = c.Search(context.Background(), "security")
	if err != nil || len(hits) != 1 || hits[0].Source != "anthropics/claude-plugins-official" {
		t.Fatalf("security hits = %+v %v", hits, err)
	}
	c.Marketplaces = func(context.Context) []string { return []string{"nobody/none"} }
	if _, err := c.Search(context.Background(), "x"); err == nil {
		t.Fatal("search with only failing marketplaces should error")
	}
}
