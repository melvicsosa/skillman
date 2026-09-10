package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/melvicsosa/skillman/internal/domain"
)

// RegisterProject records root as a project and scans it. The root must be
// an existing directory containing a .git entry or at least one per-project
// skills dir of any supported agent.
func (s *Service) RegisterProject(ctx context.Context, root string) (domain.Project, ScanSummary, error) {
	abs, err := cleanRoot(root)
	if err != nil {
		return domain.Project{}, ScanSummary{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return domain.Project{}, ScanSummary{}, fmt.Errorf("project root: %w", err)
	}
	if !info.IsDir() {
		return domain.Project{}, ScanSummary{}, fmt.Errorf("project root %s is not a directory", abs)
	}
	if !s.looksLikeProject(abs) {
		return domain.Project{}, ScanSummary{}, fmt.Errorf("%s has neither .git nor a per-project skills dir (%v)", abs, s.projectDirNames())
	}
	p, err := s.projects.Add(ctx, domain.Project{Root: abs, Name: filepath.Base(abs)})
	if err != nil {
		return domain.Project{}, ScanSummary{}, err
	}
	sum, err := s.Scan(ctx, ScanOptions{ProjectRoot: &abs})
	return p, sum, err
}

// ListProjects returns the registered projects.
func (s *Service) ListProjects(ctx context.Context) ([]domain.Project, error) {
	return s.projects.List(ctx)
}

// RemoveProject unregisters a project given its root path or numeric ID.
// Cached skill rows go with it; nothing on disk is touched.
func (s *Service) RemoveProject(ctx context.Context, ref string) (domain.Project, error) {
	p, err := s.findProject(ctx, ref)
	if err != nil {
		return domain.Project{}, err
	}
	if err := s.projects.Remove(ctx, p.ID); err != nil {
		return domain.Project{}, err
	}
	return p, nil
}

func (s *Service) findProject(ctx context.Context, ref string) (domain.Project, error) {
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil {
		all, err := s.projects.List(ctx)
		if err != nil {
			return domain.Project{}, err
		}
		for _, p := range all {
			if p.ID == id {
				return p, nil
			}
		}
	}
	abs, err := cleanRoot(ref)
	if err != nil {
		return domain.Project{}, err
	}
	p, err := s.projects.GetByRoot(ctx, abs)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Project{}, fmt.Errorf("project %s: %w", ref, domain.ErrNotFound)
	}
	return p, err
}

func (s *Service) projectDirNames() []string {
	seen := map[string]bool{}
	var out []string
	for _, a := range s.agents.Agents() {
		for _, d := range a.ProjectDirs {
			if !seen[d] {
				seen[d] = true
				out = append(out, d)
			}
		}
	}
	return out
}

func (s *Service) looksLikeProject(root string) bool {
	if _, err := os.Lstat(filepath.Join(root, ".git")); err == nil {
		return true
	}
	for _, d := range s.projectDirNames() {
		if info, err := os.Stat(filepath.Join(root, d)); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}
