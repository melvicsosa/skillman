// Package skillssh is the skills.sh registry client.
//
// Observed on 2026-09-10: every /api/v1/* endpoint (search, leaderboard,
// curated, skill detail) answers 401 without a Vercel OIDC bearer token.
// Only the legacy /api/search?q= endpoint is public. The client therefore
// uses the legacy search when no token is configured, and downloads skill
// files through the GitHub tarball of the skill's source repository unless
// a token unlocks the v1 detail endpoint (which returns files[] inline).
package skillssh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/domain"
)

// ID is the registry identifier; refs look like "skillssh:<source>/<skill>".
const ID = "skillssh"

// Prefix marks a skills.sh ref.
const Prefix = ID + ":"

// RepoFetcher downloads a whole GitHub repository ("owner/repo") into a temp
// dir; the skills.sh client uses it to locate skill files without a token.
type RepoFetcher interface {
	Fetch(ctx context.Context, ref string) (domain.FetchResult, error)
}

// Client talks to skills.sh.
type Client struct {
	HTTP    *http.Client
	BaseURL string // default https://skills.sh
	// Token is a Vercel OIDC token for the /api/v1 endpoints (optional).
	Token string
	// Repos fetches GitHub repositories for token-less downloads (optional).
	Repos RepoFetcher
}

var (
	_ domain.Registry       = (*Client)(nil)
	_ domain.TrendingLister = (*Client)(nil)
	_ domain.CuratedLister  = (*Client)(nil)
)

// New returns a client with defaults.
func New(token string, repos RepoFetcher) *Client {
	return &Client{HTTP: &http.Client{Timeout: 60 * time.Second}, BaseURL: "https://skills.sh", Token: token, Repos: repos}
}

// ID returns "skillssh".
func (*Client) ID() string { return ID }

// ParseRef splits "skillssh:owner/repo/skill" into source and slug.
func ParseRef(ref string) (source, slug string, ok bool) {
	ref = strings.TrimSpace(ref)
	if !strings.HasPrefix(ref, Prefix) {
		return "", "", false
	}
	id := strings.Trim(strings.TrimPrefix(ref, Prefix), "/")
	i := strings.LastIndex(id, "/")
	if i <= 0 || i == len(id)-1 {
		return "", "", false
	}
	return id[:i], id[i+1:], true
}

// v1Skill is the skill object of every v1 listing (docs at /docs/api).
type v1Skill struct {
	ID         string `json:"id"`
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Source     string `json:"source"`
	Installs   int    `json:"installs"`
	SourceType string `json:"sourceType"`
	InstallURL string `json:"installUrl"`
	URL        string `json:"url"`
}

func (s v1Skill) toResult() domain.RegistryResult {
	slug := s.Slug
	if slug == "" {
		slug = s.Name
	}
	id := s.ID
	if id == "" {
		id = s.Source + "/" + slug
	}
	u := s.URL
	if u == "" {
		u = "https://skills.sh/" + id
	}
	return domain.RegistryResult{
		Registry: ID, ID: id, Name: s.Name, Source: s.Source, Installs: s.Installs,
		URL: u, InstallURL: s.InstallURL, Ref: Prefix + id,
	}
}

func (c *Client) get(ctx context.Context, p string, q url.Values, out any) error {
	u := c.BaseURL + p
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "skillman")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		var apiErr struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &apiErr)
		switch {
		case resp.StatusCode == http.StatusUnauthorized || apiErr.Error == "authentication_required":
			return fmt.Errorf("skills.sh %s: %w (set SKILLSSH_TOKEN or the skillssh_token setting to a Vercel OIDC token)", p, domain.ErrAuthRequired)
		case resp.StatusCode == http.StatusNotFound:
			return fmt.Errorf("skills.sh %s: %w", p, domain.ErrNotFound)
		}
		msg := apiErr.Message
		if msg == "" {
			msg = strings.TrimSpace(string(body))
		}
		return fmt.Errorf("skills.sh %s: HTTP %d: %s", p, resp.StatusCode, msg)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("skills.sh %s: decode: %w", p, err)
	}
	return nil
}

// Search uses /api/v1/skills/search with a token and the public legacy
// /api/search otherwise.
func (c *Client) Search(ctx context.Context, q string) ([]domain.RegistryResult, error) {
	q = strings.TrimSpace(q)
	if len(q) < 2 {
		return nil, errors.New("query must be at least 2 characters")
	}
	if c.Token != "" {
		var body struct {
			Data []v1Skill `json:"data"`
		}
		if err := c.get(ctx, "/api/v1/skills/search", url.Values{"q": {q}, "limit": {"50"}}, &body); err != nil {
			return nil, err
		}
		return toResults(body.Data), nil
	}
	var body struct {
		Skills []struct {
			ID       string `json:"id"`
			SkillID  string `json:"skillId"`
			Name     string `json:"name"`
			Installs int    `json:"installs"`
			Source   string `json:"source"`
		} `json:"skills"`
	}
	if err := c.get(ctx, "/api/search", url.Values{"q": {q}}, &body); err != nil {
		return nil, err
	}
	out := make([]domain.RegistryResult, 0, len(body.Skills))
	for _, s := range body.Skills {
		out = append(out, v1Skill{ID: s.ID, Slug: s.SkillID, Name: s.Name, Source: s.Source, Installs: s.Installs,
			InstallURL: "https://github.com/" + s.Source}.toResult())
	}
	return out, nil
}

// Trending lists /api/v1/skills?view=trending (token required).
func (c *Client) Trending(ctx context.Context) ([]domain.RegistryResult, error) {
	var body struct {
		Data []v1Skill `json:"data"`
	}
	if err := c.get(ctx, "/api/v1/skills", url.Values{"view": {"trending"}, "per_page": {"50"}}, &body); err != nil {
		return nil, err
	}
	return toResults(body.Data), nil
}

// Curated lists /api/v1/skills/curated (token required).
func (c *Client) Curated(ctx context.Context) ([]domain.CuratedOwner, error) {
	var body struct {
		Data []struct {
			Owner         string    `json:"owner"`
			TotalInstalls int       `json:"totalInstalls"`
			Skills        []v1Skill `json:"skills"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/api/v1/skills/curated", nil, &body); err != nil {
		return nil, err
	}
	out := make([]domain.CuratedOwner, 0, len(body.Data))
	for _, o := range body.Data {
		out = append(out, domain.CuratedOwner{Owner: o.Owner, TotalInstalls: o.TotalInstalls, Skills: toResults(o.Skills)})
	}
	return out, nil
}

func toResults(in []v1Skill) []domain.RegistryResult {
	out := make([]domain.RegistryResult, 0, len(in))
	for _, s := range in {
		out = append(out, s.toResult())
	}
	return out
}

// Fetch downloads "skillssh:<source>/<skill>". With a token it reads files[]
// from the v1 detail endpoint; otherwise it fetches the source repository
// from GitHub and picks the directory named after the slug that holds a
// SKILL.md (preferring skills/<slug>).
func (c *Client) Fetch(ctx context.Context, ref string) (domain.FetchResult, error) {
	source, slug, ok := ParseRef(ref)
	if !ok {
		return domain.FetchResult{}, fmt.Errorf("%q is not a skills.sh reference (want skillssh:<owner>/<repo>/<skill>)", ref)
	}
	src := domain.VaultSource{Type: domain.SourceSkillSH, Ref: ref, URL: "https://skills.sh/" + source + "/" + slug}
	if c.Token != "" {
		return c.fetchV1(ctx, source, slug, src)
	}
	if c.Repos == nil {
		return domain.FetchResult{}, fmt.Errorf("skills.sh detail endpoint: %w", domain.ErrAuthRequired)
	}
	if strings.Count(source, "/") != 1 {
		return domain.FetchResult{}, fmt.Errorf("skills.sh source %q is not a GitHub repository; a token is required", source)
	}
	repo, err := c.Repos.Fetch(ctx, source)
	if err != nil {
		return domain.FetchResult{}, err
	}
	dir, rel, err := findSkillDir(repo.Dir, slug)
	if err != nil {
		repo.Cleanup()
		return domain.FetchResult{}, err
	}
	src.Subpath = rel
	src.Commit = repo.Source.Commit
	return domain.FetchResult{Dir: dir, Name: slug, Source: src, Cleanup: repo.Cleanup}, nil
}

func (c *Client) fetchV1(ctx context.Context, source, slug string, src domain.VaultSource) (domain.FetchResult, error) {
	var body struct {
		Hash  string `json:"hash"`
		Files []struct {
			Path     string `json:"path"`
			Contents string `json:"contents"`
		} `json:"files"`
	}
	if err := c.get(ctx, "/api/v1/skills/"+source+"/"+slug, nil, &body); err != nil {
		return domain.FetchResult{}, err
	}
	if len(body.Files) == 0 {
		return domain.FetchResult{}, fmt.Errorf("skills.sh %s/%s has no file snapshot: %w", source, slug, domain.ErrNotFound)
	}
	dir, err := os.MkdirTemp("", "skillman-skillssh-*")
	if err != nil {
		return domain.FetchResult{}, err
	}
	for _, f := range body.Files {
		rel := filepath.FromSlash(strings.TrimPrefix(f.Path, "/"))
		if rel == "" || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			continue
		}
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			os.RemoveAll(dir)
			return domain.FetchResult{}, err
		}
		if err := os.WriteFile(p, []byte(f.Contents), 0o644); err != nil {
			os.RemoveAll(dir)
			return domain.FetchResult{}, err
		}
	}
	src.Commit = body.Hash
	return domain.FetchResult{Dir: dir, Name: slug, Source: src, Cleanup: func() { os.RemoveAll(dir) }}, nil
}

// findSkillDir locates the directory named slug containing SKILL.md under
// root, preferring skills/<slug> and the shallowest match otherwise.
func findSkillDir(root, slug string) (dir, rel string, err error) {
	if p := filepath.Join(root, "skills", slug); fileExists(filepath.Join(p, skillfs.SkillFile)) {
		return p, "skills/" + slug, nil
	}
	if fileExists(filepath.Join(root, slug, skillfs.SkillFile)) {
		return filepath.Join(root, slug), slug, nil
	}
	var best string
	bestDepth := -1
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if d.IsDir() && d.Name() == slug && fileExists(filepath.Join(p, skillfs.SkillFile)) {
			depth := strings.Count(p, string(filepath.Separator))
			if bestDepth < 0 || depth < bestDepth {
				best, bestDepth = p, depth
			}
			return filepath.SkipDir
		}
		return nil
	})
	if best == "" {
		// skills.sh slugs come from the SKILL.md "name" field, which may differ
		// from the directory name (e.g. vercel-react-best-practices lives in
		// skills/react-best-practices). Match on the declared name.
		best = findByDeclaredName(root, slug)
	}
	if best == "" {
		return "", "", fmt.Errorf("skill %q: no directory with SKILL.md found in the source repository: %w", slug, domain.ErrNotFound)
	}
	r, _ := filepath.Rel(root, best)
	return best, filepath.ToSlash(r), nil
}

// findByDeclaredName returns the shallowest directory whose SKILL.md declares
// name equal to slug, or "".
func findByDeclaredName(root, slug string) string {
	var best string
	bestDepth := -1
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if d.IsDir() || d.Name() != skillfs.SkillFile {
			return nil
		}
		dir := filepath.Dir(p)
		fm, _, err := skillfs.ReadSkillMD(dir)
		if err != nil || fm.Name != slug {
			return nil
		}
		depth := strings.Count(dir, string(filepath.Separator))
		if bestDepth < 0 || depth < bestDepth {
			best, bestDepth = dir, depth
		}
		return nil
	})
	return best
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// Matches reports whether ref starts with "skillssh:".
func (*Client) Matches(ref string) bool {
	_, _, ok := ParseRef(ref)
	return ok
}
