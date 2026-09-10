package fs

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"unicode/utf8"

	"github.com/melvicsosa/skillman/internal/domain"
)

// NameRE is the Agent Skills spec rule for skill names.
var NameRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

const (
	maxNameLen        = 64
	maxDescriptionLen = 1024
)

// Inspector implements domain.SkillInspector on the real filesystem.
type Inspector struct{}

var _ domain.SkillInspector = Inspector{}

// Inspect parses SKILL.md, hashes the directory and validates the
// frontmatter against the spec. Spec problems are reported in SpecIssues;
// only I/O failures are returned as errors (a missing or unparsable SKILL.md
// still yields a hash and issues).
func (Inspector) Inspect(path string) (domain.SkillInfo, error) {
	var info domain.SkillInfo
	fm, _, err := ReadSkillMD(path)
	switch {
	case err == nil:
	case errors.Is(err, ErrNoFrontmatter):
		info.SpecIssues = append(info.SpecIssues, "SKILL.md has no YAML frontmatter")
	default:
		return info, fmt.Errorf("read %s: %w", path, err)
	}
	info.Name = fm.Name
	info.Description = fm.Description
	info.Version = fm.Version()
	info.SpecIssues = append(info.SpecIssues, ValidateFrontmatter(fm, filepath.Base(path))...)

	hash, err := HashDir(path)
	if err != nil {
		return info, err
	}
	info.ContentHash = hash
	return info, nil
}

// ValidateFrontmatter returns the spec violations of fm for a skill living
// in a directory named dirName.
func ValidateFrontmatter(fm Frontmatter, dirName string) []string {
	var issues []string
	if len(fm.Raw) == 0 {
		return issues // already reported as missing frontmatter
	}
	switch {
	case fm.Name == "":
		issues = append(issues, "frontmatter is missing 'name'")
	case utf8.RuneCountInString(fm.Name) > maxNameLen:
		issues = append(issues, fmt.Sprintf("name %q exceeds %d characters", fm.Name, maxNameLen))
	case !NameRE.MatchString(fm.Name):
		issues = append(issues, fmt.Sprintf("name %q must match %s", fm.Name, NameRE.String()))
	case fm.Name != dirName:
		issues = append(issues, fmt.Sprintf("name %q does not match directory name %q", fm.Name, dirName))
	}
	switch {
	case fm.Description == "":
		issues = append(issues, "frontmatter is missing 'description'")
	case utf8.RuneCountInString(fm.Description) > maxDescriptionLen:
		issues = append(issues, fmt.Sprintf("description exceeds %d characters", maxDescriptionLen))
	}
	return issues
}
