package agents

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/domain"
)

// Source implements domain.AgentSource over the static Registry.
type Source struct{}

var _ domain.AgentSource = Source{}

// Agents returns the registry as domain agents with expanded paths.
func (Source) Agents() []domain.Agent {
	out := make([]domain.Agent, 0, len(Registry))
	for _, s := range Registry {
		out = append(out, s.ToDomain())
	}
	return out
}

// ProjectDirs returns the absolute per-project skill dirs of agent under root.
func (Source) ProjectDirs(agent domain.Agent, root string) []string {
	s, ok := Lookup(agent.ID)
	if !ok {
		return nil
	}
	return s.ResolvedProjectDirs(root)
}

// Exists reports whether at least one of the agent's global dirs exists.
func (Source) Exists(agent domain.Agent) bool {
	s, ok := Lookup(agent.ID)
	return ok && s.Exists()
}

// Discover lists the skill directories agent loads in scope.
func (Source) Discover(agent domain.Agent, scope domain.Scope, projectRoot string) ([]domain.DiscoveredSkill, error) {
	s, ok := Lookup(agent.ID)
	if !ok {
		return nil, fmt.Errorf("unknown agent %q", agent.ID)
	}
	return s.Discover(scope, projectRoot)
}

// Discover lists skills of this agent in scope. Dirs that resolve to the
// same location (for example a symlinked alias dir) are only visited once.
func (s Spec) Discover(scope domain.Scope, projectRoot string) ([]domain.DiscoveredSkill, error) {
	if s.ID == PluginsAgentID {
		if scope != domain.GlobalScope {
			return nil, nil
		}
		return discoverPlugins(s.ResolvedGlobalDirs()[0])
	}
	var dirs []string
	if scope == domain.GlobalScope {
		dirs = s.ResolvedGlobalDirs()
	} else {
		if projectRoot == "" {
			return nil, errors.New("project root required for project scope")
		}
		dirs = s.ResolvedProjectDirs(projectRoot)
	}
	var out []domain.DiscoveredSkill
	visited := map[string]bool{}
	for _, dir := range dirs {
		key := dir
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			key = resolved
		}
		if visited[key] {
			continue
		}
		visited[key] = true
		found, err := skillfs.DiscoverSkills(dir)
		if err != nil {
			return nil, err
		}
		for _, d := range found {
			out = append(out, toDomain(d, false))
		}
	}
	return out, nil
}

// discoverPlugins walks <cache>/<marketplace>/<plugin>/<version>/skills/<name>.
func discoverPlugins(cache string) ([]domain.DiscoveredSkill, error) {
	marketplaces, err := os.ReadDir(cache)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read plugin cache %s: %w", cache, err)
	}
	var out []domain.DiscoveredSkill
	for _, m := range marketplaces {
		if !m.IsDir() || m.Name()[0] == '.' {
			continue
		}
		plugins, err := os.ReadDir(filepath.Join(cache, m.Name()))
		if err != nil {
			continue
		}
		for _, p := range plugins {
			if !p.IsDir() || p.Name()[0] == '.' {
				continue
			}
			versions, err := os.ReadDir(filepath.Join(cache, m.Name(), p.Name()))
			if err != nil {
				continue
			}
			for _, v := range versions {
				if !v.IsDir() || v.Name()[0] == '.' {
					continue
				}
				skillsDir := filepath.Join(cache, m.Name(), p.Name(), v.Name(), "skills")
				found, err := skillfs.DiscoverSkills(skillsDir)
				if err != nil {
					return nil, err
				}
				for _, d := range found {
					out = append(out, toDomain(d, true))
				}
			}
		}
	}
	return out, nil
}

func toDomain(d skillfs.Discovered, readOnly bool) domain.DiscoveredSkill {
	return domain.DiscoveredSkill{
		Name:       d.Name,
		Path:       d.Path,
		IsSymlink:  d.IsSymlink,
		LinkTarget: d.LinkTarget,
		Dangling:   d.Dangling,
		ReadOnly:   readOnly,
	}
}
