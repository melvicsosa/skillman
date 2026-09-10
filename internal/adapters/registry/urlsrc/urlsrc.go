// Package urlsrc downloads a skill from a direct URL: a .zip/.tar.gz
// archive or a raw SKILL.md file.
package urlsrc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/adapters/registry/local"
	"github.com/melvicsosa/skillman/internal/domain"
)

// ID is the registry identifier.
const ID = "url"

// Client implements domain.Registry for direct URLs.
type Client struct {
	HTTP *http.Client
}

var _ domain.Registry = (*Client)(nil)

// New returns a client with a timeout.
func New() *Client { return &Client{HTTP: &http.Client{Timeout: 2 * time.Minute}} }

// ID returns "url".
func (*Client) ID() string { return ID }

// Search is not supported.
func (*Client) Search(context.Context, string) ([]domain.RegistryResult, error) {
	return nil, fmt.Errorf("direct URLs: search %w", domain.ErrUnsupported)
}

// Looks reports whether ref is an http(s) URL this client handles.
func Looks(ref string) bool {
	u, err := url.Parse(strings.TrimSpace(ref))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	return local.IsArchive(u.Path) || path.Base(u.Path) == skillfs.SkillFile
}

// Fetch downloads ref into a temp dir.
func (c *Client) Fetch(ctx context.Context, ref string) (domain.FetchResult, error) {
	ref = strings.TrimSpace(ref)
	u, err := url.Parse(ref)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return domain.FetchResult{}, fmt.Errorf("%q is not an http(s) URL", ref)
	}
	if !Looks(ref) {
		return domain.FetchResult{}, fmt.Errorf("%q: only .zip, .tar.gz and SKILL.md URLs are supported", ref)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ref, nil)
	if err != nil {
		return domain.FetchResult{}, err
	}
	req.Header.Set("User-Agent", "skillman")
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return domain.FetchResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return domain.FetchResult{}, fmt.Errorf("%s: %w", ref, domain.ErrNotFound)
	}
	if resp.StatusCode >= 400 {
		return domain.FetchResult{}, fmt.Errorf("%s: HTTP %d", ref, resp.StatusCode)
	}
	tmp, err := os.MkdirTemp("", "skillman-url-*")
	if err != nil {
		return domain.FetchResult{}, err
	}
	cleanup := func() { os.RemoveAll(tmp) }
	src := domain.VaultSource{Type: domain.SourceURL, Ref: ref, URL: ref}
	var (
		dir  string
		name string
	)
	if path.Base(u.Path) == skillfs.SkillFile {
		// SKILL.md URL: the parent path segment names the skill.
		name = path.Base(path.Dir(u.Path))
		if name == "/" || name == "." || name == "" {
			name = "skill"
		}
		dir = filepath.Join(tmp, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			cleanup()
			return domain.FetchResult{}, err
		}
		f, err := os.Create(filepath.Join(dir, skillfs.SkillFile))
		if err != nil {
			cleanup()
			return domain.FetchResult{}, err
		}
		_, err = io.Copy(f, io.LimitReader(resp.Body, 16<<20))
		f.Close()
		if err != nil {
			cleanup()
			return domain.FetchResult{}, err
		}
	} else {
		dir, name, err = local.ExtractInto(resp.Body, path.Base(u.Path), tmp)
		if err != nil {
			cleanup()
			return domain.FetchResult{}, err
		}
	}
	return domain.FetchResult{Dir: dir, Name: name, Source: src, Cleanup: cleanup}, nil
}

// Matches reports whether ref is a supported direct URL.
func (*Client) Matches(ref string) bool { return Looks(ref) }
