// Package marketplace resolves Claude plugin marketplaces: GitHub
// repositories holding .claude-plugin/marketplace.json (PLAN.md 1.3). A
// ref looks like "marketplace:<owner>/<repo>/<plugin>"; the plugin's
// source is resolved to a GitHub ref and downloaded through the GitHub
// registry, so the converter's plugin unpack path stores each skill.
package marketplace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/melvicsosa/skillman/internal/domain"
)

// ID is the registry identifier.
const ID = "marketplace"

// Prefix marks a marketplace ref.
const Prefix = ID + ":"

// ManifestPath is where a marketplace repository keeps its manifest.
const ManifestPath = ".claude-plugin/marketplace.json"

// Manifest is the shape of marketplace.json as published by
// anthropics/claude-plugins-official: name, description, owner, optional
// renames (old plugin name -> new) and the plugins list.
type Manifest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Owner       Owner             `json:"owner"`
	Renames     map[string]string `json:"renames,omitempty"`
	Plugins     []Plugin          `json:"plugins"`
}

// Owner is the marketplace maintainer.
type Owner struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

// Plugin is one marketplace entry. Source is either a string (a path
// relative to the marketplace repository such as "./plugins/foo") or an
// object (see PluginSource).
type Plugin struct {
	Name        string       `json:"name"`
	DisplayName string       `json:"displayName,omitempty"`
	Description string       `json:"description"`
	Version     string       `json:"version,omitempty"`
	Category    string       `json:"category,omitempty"`
	Keywords    []string     `json:"keywords,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	Homepage    string       `json:"homepage,omitempty"`
	Author      Owner        `json:"author,omitempty"`
	Source      PluginSource `json:"source"`
	// Skills lists skill directories relative to the plugin root when the
	// plugin keeps them outside skills/ ("./local-ai-use").
	Skills []string `json:"skills,omitempty"`
}

// PluginSource is the "source" field. Observed forms:
//
//	"./plugins/foo"                                   relative path in the marketplace repo
//	{"source":"url","url":"https://github.com/o/r.git","sha":"..."}
//	{"source":"git-subdir","url":"https://github.com/o/r.git","path":"plugins/x","ref":"v1.2.3","sha":"..."}
//	{"source":"github","repo":"o/r","path":"...","ref":"..."}   (documented by Claude Code)
type PluginSource struct {
	Path string // relative path when the source is a string
	Kind string `json:"source"`
	URL  string `json:"url,omitempty"`
	Repo string `json:"repo,omitempty"`
	Dir  string `json:"path,omitempty"`
	Ref  string `json:"ref,omitempty"`
	Sha  string `json:"sha,omitempty"`
}

// UnmarshalJSON accepts both the string and the object form.
func (p *PluginSource) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*p = PluginSource{Path: s}
		return nil
	}
	type raw PluginSource
	var r raw
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*p = PluginSource(r)
	return nil
}

// MarshalJSON renders the string form back when it was a string.
func (p PluginSource) MarshalJSON() ([]byte, error) {
	if p.Path != "" && p.Kind == "" {
		return json.Marshal(p.Path)
	}
	type raw PluginSource
	return json.Marshal(raw(p))
}

// Ref is a parsed marketplace reference.
type Ref struct {
	Owner  string
	Repo   string
	Plugin string // "" when the ref names the whole marketplace
}

// Marketplace is "owner/repo".
func (r Ref) Marketplace() string { return r.Owner + "/" + r.Repo }

// String renders the canonical "marketplace:owner/repo/plugin" form.
func (r Ref) String() string {
	s := Prefix + r.Marketplace()
	if r.Plugin != "" {
		s += "/" + r.Plugin
	}
	return s
}

var segmentRE = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// ParseRef accepts "marketplace:owner/repo[/plugin]".
func ParseRef(ref string) (Ref, bool) {
	ref = strings.TrimSpace(ref)
	if !strings.HasPrefix(ref, Prefix) {
		return Ref{}, false
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(ref, Prefix), "/"), "/")
	if len(parts) < 2 || len(parts) > 3 {
		return Ref{}, false
	}
	r := Ref{Owner: parts[0], Repo: parts[1]}
	if len(parts) == 3 {
		r.Plugin = parts[2]
	}
	for _, seg := range parts {
		if !segmentRE.MatchString(seg) {
			return Ref{}, false
		}
	}
	return r, true
}

// Fetcher is the part of the GitHub client the marketplace needs.
type Fetcher interface {
	RawFile(ctx context.Context, owner, repo, ref, filePath string) ([]byte, error)
	Fetch(ctx context.Context, ref string) (domain.FetchResult, error)
}

// Client resolves marketplace refs and searches configured marketplaces.
type Client struct {
	GitHub Fetcher
	// Marketplaces returns the "owner/repo" list to search (read on every
	// call so settings changes apply without a restart).
	Marketplaces func(ctx context.Context) []string
	// CacheTTL bounds how long a fetched manifest is reused.
	CacheTTL time.Duration

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	manifest Manifest
	at       time.Time
}

var _ domain.Registry = (*Client)(nil)

// New returns a client backed by gh and the marketplaces function.
func New(gh Fetcher, marketplaces func(ctx context.Context) []string) *Client {
	return &Client{GitHub: gh, Marketplaces: marketplaces, CacheTTL: 10 * time.Minute}
}

// ID returns "marketplace".
func (*Client) ID() string { return ID }

// Matches reports whether ref has the marketplace form.
func (*Client) Matches(ref string) bool {
	_, ok := ParseRef(ref)
	return ok
}

// Load fetches (or reuses) the manifest of marketplace "owner/repo".
func (c *Client) Load(ctx context.Context, marketplace string) (Manifest, error) {
	owner, repo, ok := strings.Cut(strings.Trim(marketplace, "/"), "/")
	if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return Manifest{}, fmt.Errorf("marketplace %q: want owner/repo", marketplace)
	}
	key := owner + "/" + repo
	c.mu.Lock()
	if hit, ok := c.cache[key]; ok && c.CacheTTL > 0 && time.Since(hit.at) < c.CacheTTL {
		c.mu.Unlock()
		return hit.manifest, nil
	}
	c.mu.Unlock()
	b, err := c.GitHub.RawFile(ctx, owner, repo, "", ManifestPath)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return Manifest{}, fmt.Errorf("%s has no %s: %w", key, ManifestPath, domain.ErrNotFound)
		}
		return Manifest{}, err
	}
	m, err := Parse(b)
	if err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", key, err)
	}
	c.mu.Lock()
	if c.cache == nil {
		c.cache = map[string]cached{}
	}
	c.cache[key] = cached{manifest: m, at: time.Now()}
	c.mu.Unlock()
	return m, nil
}

// Parse decodes a marketplace.json.
func Parse(b []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("decode %s: %w", ManifestPath, err)
	}
	if len(m.Plugins) == 0 {
		return m, fmt.Errorf("%s lists no plugins", ManifestPath)
	}
	return m, nil
}

// Find returns the plugin named name, following renames.
func (m Manifest) Find(name string) (Plugin, bool) {
	if renamed, ok := m.Renames[name]; ok {
		name = renamed
	}
	for _, p := range m.Plugins {
		if p.Name == name {
			return p, true
		}
	}
	return Plugin{}, false
}

// GitHubRef resolves the plugin source to a ref the GitHub registry
// accepts ("owner/repo[/path][@ref]"). marketplace is the "owner/repo" the
// manifest came from, used for relative sources.
func (p Plugin) GitHubRef(marketplace string) (string, error) {
	src := p.Source
	if src.Kind == "" {
		rel := strings.TrimPrefix(path.Clean("/"+strings.TrimPrefix(src.Path, "./")), "/")
		if rel == "" || rel == "." {
			return marketplace, nil
		}
		return marketplace + "/" + rel, nil
	}
	repo := strings.Trim(src.Repo, "/")
	if repo == "" && src.URL != "" {
		u, err := url.Parse(src.URL)
		if err != nil || !strings.EqualFold(u.Host, "github.com") {
			return "", fmt.Errorf("plugin %s: source %s is not on github.com: %w", p.Name, src.URL, domain.ErrUnsupported)
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 2 {
			return "", fmt.Errorf("plugin %s: cannot parse source url %s", p.Name, src.URL)
		}
		repo = parts[0] + "/" + strings.TrimSuffix(parts[1], ".git")
	}
	if repo == "" {
		return "", fmt.Errorf("plugin %s: source kind %q has no repository: %w", p.Name, src.Kind, domain.ErrUnsupported)
	}
	ref := repo
	if dir := strings.Trim(src.Dir, "/"); dir != "" {
		ref += "/" + dir
	}
	switch {
	case src.Ref != "":
		ref += "@" + src.Ref
	case src.Sha != "":
		ref += "@" + src.Sha
	}
	return ref, nil
}

// Fetch implements domain.Registry: resolve the plugin and download it
// through GitHub. The result keeps the marketplace ref as provenance.
func (c *Client) Fetch(ctx context.Context, ref string) (domain.FetchResult, error) {
	r, ok := ParseRef(ref)
	if !ok {
		return domain.FetchResult{}, fmt.Errorf("%q is not a marketplace reference (want marketplace:<owner>/<repo>/<plugin>)", ref)
	}
	m, err := c.Load(ctx, r.Marketplace())
	if err != nil {
		return domain.FetchResult{}, err
	}
	if r.Plugin == "" {
		return domain.FetchResult{}, fmt.Errorf("%s lists %d plugins; pick one: %s", r.Marketplace(), len(m.Plugins), strings.Join(pluginNames(m, 8), ", "))
	}
	p, ok := m.Find(r.Plugin)
	if !ok {
		return domain.FetchResult{}, fmt.Errorf("marketplace %s has no plugin %q: %w", r.Marketplace(), r.Plugin, domain.ErrNotFound)
	}
	ghRef, err := p.GitHubRef(r.Marketplace())
	if err != nil {
		return domain.FetchResult{}, err
	}
	res, err := c.GitHub.Fetch(ctx, ghRef)
	if err != nil {
		return domain.FetchResult{}, fmt.Errorf("plugin %s (%s): %w", p.Name, ghRef, err)
	}
	res.Name = p.Name
	res.Source = domain.VaultSource{
		Type: domain.SourceMarketplace, Ref: r.String(), URL: res.Source.URL,
		Subpath: res.Source.Subpath, Commit: res.Source.Commit,
	}
	if p.Source.Sha != "" {
		res.Source.Commit = p.Source.Sha
	}
	return res, nil
}

func pluginNames(m Manifest, n int) []string {
	var out []string
	for i, p := range m.Plugins {
		if i == n {
			out = append(out, "...")
			break
		}
		out = append(out, p.Name)
	}
	return out
}

// Search lists plugins of every configured marketplace whose name,
// display name, description, keywords or tags contain q. A marketplace
// that fails to load is skipped; the error is returned only when every
// marketplace failed.
func (c *Client) Search(ctx context.Context, q string) ([]domain.RegistryResult, error) {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil, errors.New("empty query")
	}
	var marketplaces []string
	if c.Marketplaces != nil {
		marketplaces = c.Marketplaces(ctx)
	}
	if len(marketplaces) == 0 {
		return nil, errors.New("no marketplaces configured (set the marketplaces setting)")
	}
	var (
		out  []domain.RegistryResult
		errs []string
	)
	for _, mk := range marketplaces {
		m, err := c.Load(ctx, mk)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		for _, p := range m.Plugins {
			if !p.matches(q) {
				continue
			}
			out = append(out, c.result(mk, p))
		}
	}
	if len(errs) == len(marketplaces) {
		return nil, errors.New(strings.Join(errs, "; "))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// matches reports whether every whitespace-separated term of q occurs in
// the plugin's name, display name, description, keywords or tags.
func (p Plugin) matches(q string) bool {
	hay := strings.ToLower(strings.Join(append(append([]string{p.Name, p.DisplayName, p.Description}, p.Keywords...), p.Tags...), "\n"))
	for _, term := range strings.Fields(q) {
		if !strings.Contains(hay, term) {
			return false
		}
	}
	return true
}

func (c *Client) result(marketplace string, p Plugin) domain.RegistryResult {
	r := Ref{Plugin: p.Name}
	r.Owner, r.Repo, _ = strings.Cut(marketplace, "/")
	u := p.Homepage
	if u == "" {
		u = "https://github.com/" + marketplace
		if p.Source.Kind == "" && p.Source.Path != "" {
			u += "/tree/HEAD/" + strings.TrimPrefix(path.Clean(p.Source.Path), "./")
		} else if p.Source.URL != "" {
			u = strings.TrimSuffix(p.Source.URL, ".git")
		}
	}
	return domain.RegistryResult{
		Registry: ID, ID: marketplace + "/" + p.Name, Name: p.Name, Source: marketplace,
		Description: p.Description, URL: u, Ref: r.String(),
	}
}
