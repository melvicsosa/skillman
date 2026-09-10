// Package github fetches skills from GitHub repositories via the tarball
// API (PLAN.md section 1.3) and searches repositories.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/melvicsosa/skillman/internal/adapters/registry/archive"
	"github.com/melvicsosa/skillman/internal/domain"
)

// ID is the registry identifier.
const ID = "github"

// Ref is a parsed GitHub reference.
type Ref struct {
	Owner   string
	Repo    string
	Ref     string // branch, tag or sha; "" means the default branch
	Subpath string // path inside the repo, "" for the root
}

// String renders the canonical "owner/repo[/subpath]" form plus @ref.
func (r Ref) String() string {
	s := r.Owner + "/" + r.Repo
	if r.Subpath != "" {
		s += "/" + r.Subpath
	}
	if r.Ref != "" {
		s += "@" + r.Ref
	}
	return s
}

// RepoURL is the https URL of the repository.
func (r Ref) RepoURL() string { return "https://github.com/" + r.Owner + "/" + r.Repo }

var segmentRE = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// ParseRef accepts owner/repo, owner/repo/path, owner/repo[/path]@ref,
// https://github.com/o/r, https://github.com/o/r.git and
// https://github.com/o/r/tree/<ref>/<path>. ok is false for anything else.
func ParseRef(ref string) (Ref, bool) {
	ref = strings.TrimSpace(ref)
	if strings.HasPrefix(ref, "github:") {
		ref = strings.TrimPrefix(ref, "github:")
	}
	if strings.Contains(ref, "://") || strings.HasPrefix(ref, "github.com/") {
		return parseURL(ref)
	}
	if strings.HasPrefix(ref, ".") || strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, "~") {
		return Ref{}, false
	}
	var r Ref
	if at := strings.LastIndex(ref, "@"); at > 0 {
		r.Ref = ref[at+1:]
		ref = ref[:at]
	}
	parts := strings.Split(strings.Trim(ref, "/"), "/")
	if len(parts) < 2 {
		return Ref{}, false
	}
	r.Owner, r.Repo = parts[0], strings.TrimSuffix(parts[1], ".git")
	if !segmentRE.MatchString(r.Owner) || !segmentRE.MatchString(r.Repo) {
		return Ref{}, false
	}
	r.Subpath = path.Clean(strings.Join(parts[2:], "/"))
	if r.Subpath == "." {
		r.Subpath = ""
	}
	return r, true
}

func parseURL(raw string) (Ref, bool) {
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(u.Host, "github.com") {
		return Ref{}, false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return Ref{}, false
	}
	r := Ref{Owner: parts[0], Repo: strings.TrimSuffix(parts[1], ".git")}
	if !segmentRE.MatchString(r.Owner) || !segmentRE.MatchString(r.Repo) {
		return Ref{}, false
	}
	if len(parts) >= 4 && (parts[2] == "tree" || parts[2] == "blob") {
		r.Ref = parts[3]
		r.Subpath = strings.Join(parts[4:], "/")
		if parts[2] == "blob" && strings.HasSuffix(r.Subpath, "/SKILL.md") {
			r.Subpath = path.Dir(r.Subpath)
		}
	} else if len(parts) > 2 {
		r.Subpath = strings.Join(parts[2:], "/")
	}
	if r.Subpath == "." {
		r.Subpath = ""
	}
	return r, true
}

// Client talks to the GitHub REST API.
type Client struct {
	HTTP    *http.Client
	BaseURL string // API base, default https://api.github.com
	// Token is used as a bearer token when set (raises rate limits).
	Token string
}

var _ domain.Registry = (*Client)(nil)

// New returns a client with sane defaults.
func New(token string) *Client {
	return &Client{HTTP: &http.Client{Timeout: 2 * time.Minute}, BaseURL: "https://api.github.com", Token: token}
}

// ID returns "github".
func (*Client) ID() string { return ID }

func (c *Client) do(ctx context.Context, method, u string, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
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
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := strings.TrimSpace(string(body))
		var apiErr struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Message != "" {
			msg = apiErr.Message
		}
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("github %s: %w: %s", u, domain.ErrNotFound, msg)
		}
		if resp.StatusCode == http.StatusUnauthorized || (resp.StatusCode == http.StatusForbidden && strings.Contains(strings.ToLower(msg), "rate limit")) {
			return nil, fmt.Errorf("github %s: %w: %s (set GITHUB_TOKEN or the github_token setting)", u, domain.ErrAuthRequired, msg)
		}
		return nil, fmt.Errorf("github %s: HTTP %d: %s", u, resp.StatusCode, msg)
	}
	return resp, nil
}

// DefaultBranch returns the repository's default branch, "main" on failure
// to read it from the API response.
func (c *Client) DefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s", c.BaseURL, owner, repo), "application/vnd.github+json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var meta struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil || meta.DefaultBranch == "" {
		return "main", nil
	}
	return meta.DefaultBranch, nil
}

// Download fetches the tarball of r and extracts r.Subpath into a new temp
// dir. It returns the dir, the commit sha recorded in the tarball's top
// directory name, and the ref that was actually used.
func (c *Client) Download(ctx context.Context, r Ref) (dir, commit, usedRef string, err error) {
	usedRef = r.Ref
	if usedRef == "" {
		usedRef, err = c.DefaultBranch(ctx, r.Owner, r.Repo)
		if err != nil {
			return "", "", "", err
		}
	}
	u := fmt.Sprintf("%s/repos/%s/%s/tarball/%s", c.BaseURL, r.Owner, r.Repo, url.PathEscape(usedRef))
	resp, err := c.do(ctx, http.MethodGet, u, "application/vnd.github+json")
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()
	dir, err = os.MkdirTemp("", "skillman-github-*")
	if err != nil {
		return "", "", "", err
	}
	res, err := archive.ExtractTarGz(resp.Body, dir, archive.Options{StripTop: true, Subpath: r.Subpath})
	if err != nil {
		os.RemoveAll(dir)
		if errors.Is(err, archive.ErrSubpathMissing) {
			return "", "", "", fmt.Errorf("%s: path %q: %w", r.Owner+"/"+r.Repo, r.Subpath, domain.ErrNotFound)
		}
		return "", "", "", err
	}
	if i := strings.LastIndex(res.TopDir, "-"); i >= 0 {
		commit = res.TopDir[i+1:]
	}
	return dir, commit, usedRef, nil
}

// Fetch implements domain.Registry.
func (c *Client) Fetch(ctx context.Context, ref string) (domain.FetchResult, error) {
	r, ok := ParseRef(ref)
	if !ok {
		return domain.FetchResult{}, fmt.Errorf("%q is not a GitHub reference", ref)
	}
	dir, commit, usedRef, err := c.Download(ctx, r)
	if err != nil {
		return domain.FetchResult{}, err
	}
	name := ""
	if r.Subpath != "" {
		name = path.Base(r.Subpath)
	}
	return domain.FetchResult{
		Dir:  dir,
		Name: name,
		Source: domain.VaultSource{
			Type: domain.SourceGitHub, Ref: ref, URL: r.RepoURL() + "/tree/" + usedRef + "/" + r.Subpath,
			Subpath: r.Subpath, Commit: commit,
		},
		Cleanup: func() { os.RemoveAll(dir) },
	}, nil
}

// Search looks for repositories tagged with the agent-skills topic or
// mentioning skills, returning "owner/repo" refs.
func (c *Client) Search(ctx context.Context, q string) ([]domain.RegistryResult, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, errors.New("empty query")
	}
	query := url.QueryEscape(q + " topic:agent-skills")
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("%s/search/repositories?q=%s&per_page=30", c.BaseURL, query), "application/vnd.github+json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var body struct {
		Items []struct {
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			HTMLURL     string `json:"html_url"`
			Stars       int    `json:"stargazers_count"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode github search: %w", err)
	}
	out := make([]domain.RegistryResult, 0, len(body.Items))
	for _, it := range body.Items {
		out = append(out, domain.RegistryResult{
			Registry: ID, ID: it.FullName, Name: path.Base(it.FullName), Source: it.FullName,
			Installs: it.Stars, URL: it.HTMLURL, InstallURL: it.HTMLURL, Ref: it.FullName,
		})
	}
	return out, nil
}

// Matches reports whether ref has a GitHub form (see ParseRef).
func (*Client) Matches(ref string) bool {
	_, ok := ParseRef(ref)
	return ok
}
