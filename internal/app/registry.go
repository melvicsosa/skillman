package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/melvicsosa/skillman/internal/domain"
)

// RefMatcher is implemented by registries that can tell whether a ref is
// theirs by its form alone. ResolveRef asks registries in order.
type RefMatcher interface {
	Matches(ref string) bool
}

// ResolveRef picks the registry for ref (PLAN.md section 5 ref forms).
func (s *Service) ResolveRef(ref string) (domain.Registry, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, errors.New("empty reference")
	}
	if i := strings.Index(ref, ":"); i > 0 && !strings.Contains(ref[:i], "/") && !strings.Contains(ref[:i], ".") {
		// Explicit "<registry>:<rest>" prefix (skillssh:, github:, ...).
		if reg := s.registry(ref[:i]); reg != nil {
			if m, ok := reg.(RefMatcher); !ok || m.Matches(ref) {
				return reg, nil
			}
		}
	}
	for _, reg := range s.registries {
		if m, ok := reg.(RefMatcher); ok && m.Matches(ref) {
			return reg, nil
		}
	}
	return nil, fmt.Errorf("cannot tell what %q refers to: use owner/repo[/path], a github.com URL, skillssh:<owner>/<repo>/<skill>, marketplace:<owner>/<repo>/<plugin>, a local path or a .zip/.tar.gz/SKILL.md URL", ref)
}

func (s *Service) registry(id string) domain.Registry {
	for _, reg := range s.registries {
		if reg.ID() == id {
			return reg
		}
	}
	return nil
}

// SearchResult is the merged output of a registry search.
type SearchResult struct {
	Results []domain.RegistryResult `json:"results"`
	Errors  []string                `json:"errors"`
}

// Search queries one registry (source) or every registry that supports
// search. Failures of individual registries are reported in Errors; an
// error is returned only when nothing could be searched.
func (s *Service) Search(ctx context.Context, q, source string) (SearchResult, error) {
	res := SearchResult{Results: []domain.RegistryResult{}, Errors: []string{}}
	q = strings.TrimSpace(q)
	if q == "" {
		return res, errors.New("empty query")
	}
	var regs []domain.Registry
	if source != "" {
		reg := s.registry(source)
		if reg == nil {
			return res, fmt.Errorf("unknown registry %q (want skillssh, github or marketplace)", source)
		}
		regs = []domain.Registry{reg}
	} else {
		regs = s.registries
	}
	searched := 0
	for _, reg := range regs {
		hits, err := reg.Search(ctx, q)
		if errors.Is(err, domain.ErrUnsupported) {
			continue
		}
		searched++
		if err != nil {
			res.Errors = append(res.Errors, err.Error())
			continue
		}
		res.Results = append(res.Results, hits...)
	}
	if searched == 0 {
		return res, errors.New("no registry supports search")
	}
	if len(res.Results) == 0 && len(res.Errors) == searched {
		return res, errors.New(strings.Join(res.Errors, "; "))
	}
	return res, nil
}

// Trending returns the trending list of the first registry offering one.
func (s *Service) Trending(ctx context.Context) ([]domain.RegistryResult, error) {
	for _, reg := range s.registries {
		if t, ok := reg.(domain.TrendingLister); ok {
			return t.Trending(ctx)
		}
	}
	return nil, fmt.Errorf("trending: %w", domain.ErrUnsupported)
}

// Curated returns the curated list of the first registry offering one.
func (s *Service) Curated(ctx context.Context) ([]domain.CuratedOwner, error) {
	for _, reg := range s.registries {
		if c, ok := reg.(domain.CuratedLister); ok {
			return c.Curated(ctx)
		}
	}
	return nil, fmt.Errorf("curated: %w", domain.ErrUnsupported)
}
