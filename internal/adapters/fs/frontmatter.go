// Package fs holds filesystem adapters: SKILL.md parsing, directory hashing,
// skill discovery and the quarantine mover.
package fs

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillFile is the file every skill directory must contain.
const SkillFile = "SKILL.md"

// ErrNoFrontmatter is returned when SKILL.md has no leading YAML block.
var ErrNoFrontmatter = errors.New("SKILL.md has no YAML frontmatter")

// Frontmatter is the parsed YAML header of a SKILL.md file.
type Frontmatter struct {
	Name        string
	Description string
	License     string
	Metadata    map[string]string // non-string values are stringified
	Raw         map[string]any    // every key as decoded
}

// Version returns metadata.version, falling back to a top-level "version" key.
func (f Frontmatter) Version() string {
	if v, ok := f.Metadata["version"]; ok && v != "" {
		return v
	}
	if v, ok := f.Raw["version"]; ok {
		return stringify(v)
	}
	return ""
}

// SplitFrontmatter separates content into the raw YAML header (without the
// fences) and the body. When there is no frontmatter it returns a nil
// header, the whole content as body and ErrNoFrontmatter.
func SplitFrontmatter(content []byte) (header []byte, body string, err error) {
	content = bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM
	if !bytes.HasPrefix(content, []byte("---")) {
		return nil, string(content), ErrNoFrontmatter
	}
	rest := content[3:]
	// The opening fence must be alone on its line.
	nl := bytes.IndexByte(rest, '\n')
	if nl < 0 || strings.TrimSpace(string(rest[:nl])) != "" {
		return nil, string(content), ErrNoFrontmatter
	}
	rest = rest[nl+1:]
	end := findClosingFence(rest)
	if end < 0 {
		return nil, string(content), fmt.Errorf("%w: closing fence missing", ErrNoFrontmatter)
	}
	header = rest[:end]
	tail := rest[end:]
	if nl := bytes.IndexByte(tail, '\n'); nl >= 0 {
		tail = tail[nl+1:]
	} else {
		tail = nil
	}
	return header, string(tail), nil
}

// ParseSkillMD splits content into frontmatter and body. When there is no
// frontmatter it returns an empty Frontmatter, the whole content as body and
// ErrNoFrontmatter; callers treat that as a spec issue, not a failure.
func ParseSkillMD(content []byte) (Frontmatter, string, error) {
	fm := Frontmatter{Metadata: map[string]string{}, Raw: map[string]any{}}
	header, body, err := SplitFrontmatter(content)
	if err != nil {
		return fm, body, err
	}
	if err := yaml.Unmarshal(header, &fm.Raw); err != nil {
		return fm, body, fmt.Errorf("invalid frontmatter YAML: %w", err)
	}
	if fm.Raw == nil {
		fm.Raw = map[string]any{}
	}
	fm.Name = stringify(fm.Raw["name"])
	fm.Description = stringify(fm.Raw["description"])
	fm.License = stringify(fm.Raw["license"])
	if md, ok := fm.Raw["metadata"].(map[string]any); ok {
		for k, v := range md {
			fm.Metadata[k] = stringify(v)
		}
	}
	return fm, body, nil
}

// ReadSkillMD parses <dir>/SKILL.md.
func ReadSkillMD(dir string) (Frontmatter, string, error) {
	content, err := os.ReadFile(filepath.Join(dir, SkillFile))
	if err != nil {
		return Frontmatter{Metadata: map[string]string{}, Raw: map[string]any{}}, "", err
	}
	return ParseSkillMD(content)
}

// findClosingFence returns the byte offset of a line that is exactly "---"
// (ignoring trailing whitespace), or -1.
func findClosingFence(b []byte) int {
	off := 0
	for off <= len(b) {
		nl := bytes.IndexByte(b[off:], '\n')
		var line []byte
		if nl < 0 {
			line = b[off:]
		} else {
			line = b[off : off+nl]
		}
		if strings.TrimRight(string(line), " \t\r") == "---" {
			return off
		}
		if nl < 0 {
			return -1
		}
		off += nl + 1
	}
	return -1
}

func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []any:
		parts := make([]string, 0, len(t))
		for _, e := range t {
			parts = append(parts, stringify(e))
		}
		return strings.Join(parts, ",")
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+"="+stringify(t[k]))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprint(t)
	}
}
