// Package convert detects the shape of a skill source and normalizes it to
// an Agent Skills spec directory (PLAN.md section 4). Bodies are never
// rewritten; only the YAML frontmatter is normalized.
package convert

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/domain"
)

const maxDescriptionLen = 1024

// Converter implements domain.Converter on the real filesystem.
type Converter struct{}

var _ domain.Converter = Converter{}

// RejectedError explains why a source cannot become a skill.
type RejectedError struct {
	Shape  domain.SourceShape
	Source string
	Reason string
}

func (e *RejectedError) Error() string {
	return fmt.Sprintf("%s (%s): %s", e.Source, e.Shape, e.Reason)
}

// Unwrap lets errors.Is match domain.ErrRejected.
func (e *RejectedError) Unwrap() error { return domain.ErrRejected }

func reject(shape domain.SourceShape, src, reason string) error {
	return &RejectedError{Shape: shape, Source: src, Reason: reason}
}

// Detect classifies src. Directories are spec skills (SKILL.md present) or
// Claude plugins (.claude-plugin/plugin.json plus skills/); single files are
// Cursor rules (.mdc), OpenCode commands (.md with agent/subtask keys or
// under an .opencode dir) or Claude commands (any other .md).
func (Converter) Detect(src string) (domain.SourceShape, error) {
	info, err := os.Stat(src)
	if err != nil {
		return domain.ShapeUnknown, err
	}
	if info.IsDir() {
		if fileExists(filepath.Join(src, skillfs.SkillFile)) {
			return domain.ShapeSkill, nil
		}
		if fileExists(filepath.Join(src, ".claude-plugin", "plugin.json")) && (dirExists(filepath.Join(src, "skills")) || len(pluginSkillDirs(src)) > 0) {
			return domain.ShapeClaudePlugin, nil
		}
		if fileExists(filepath.Join(src, ".claude-plugin", "marketplace.json")) {
			return domain.ShapeMarketplace, nil
		}
		return domain.ShapeUnknown, nil
	}
	switch strings.ToLower(filepath.Ext(src)) {
	case ".mdc":
		return domain.ShapeCursorRule, nil
	case ".md":
		content, err := os.ReadFile(src)
		if err != nil {
			return domain.ShapeUnknown, err
		}
		fm, _, _ := skillfs.ParseSkillMD(content)
		if _, ok := fm.Raw["agent"]; ok {
			return domain.ShapeOpenCodeCommand, nil
		}
		if _, ok := fm.Raw["subtask"]; ok {
			return domain.ShapeOpenCodeCommand, nil
		}
		if strings.Contains(filepath.ToSlash(src), "/.opencode/") {
			return domain.ShapeOpenCodeCommand, nil
		}
		return domain.ShapeClaudeCommand, nil
	}
	return domain.ShapeUnknown, nil
}

// Convert normalizes src into dstRoot/<name>. See domain.Converter.
func (c Converter) Convert(src, dstRoot, name string) (domain.ConversionReport, error) {
	shape, err := c.Detect(src)
	if err != nil {
		return domain.ConversionReport{}, err
	}
	report := domain.ConversionReport{Shape: shape, Skills: []domain.ConvertedSkill{}}
	switch shape {
	case domain.ShapeSkill:
		sk, err := convertSkillDir(src, dstRoot, name)
		if err != nil {
			return report, err
		}
		report.Skills = append(report.Skills, sk)
	case domain.ShapeClaudePlugin:
		skills, err := unpackPlugin(src, dstRoot)
		if err != nil {
			return report, err
		}
		report.Skills = skills
	case domain.ShapeCursorRule, domain.ShapeClaudeCommand, domain.ShapeOpenCodeCommand:
		sk, err := convertFile(shape, src, dstRoot, name)
		if err != nil {
			return report, err
		}
		report.Skills = append(report.Skills, sk)
	case domain.ShapeMarketplace:
		return report, reject(domain.ShapeMarketplace, src, describeMarketplace(src))
	default:
		return report, reject(domain.ShapeUnknown, src, describeUnknown(src))
	}
	return report, nil
}

// describeMarketplace explains that a marketplace repository must be added
// one plugin at a time and lists a few plugin names.
func describeMarketplace(src string) string {
	b, err := os.ReadFile(filepath.Join(src, ".claude-plugin", "marketplace.json"))
	if err != nil {
		return "a Claude marketplace, not a plugin: add one of its plugins with marketplace:<owner>/<repo>/<plugin>"
	}
	var m struct {
		Plugins []struct {
			Name string `json:"name"`
		} `json:"plugins"`
	}
	_ = json.Unmarshal(b, &m)
	names := make([]string, 0, 8)
	for _, p := range m.Plugins {
		if len(names) == 8 {
			names = append(names, "...")
			break
		}
		names = append(names, p.Name)
	}
	return fmt.Sprintf("a Claude marketplace listing %d plugins, not a plugin: add one with marketplace:<owner>/<repo>/<plugin> (%s)", len(m.Plugins), strings.Join(names, ", "))
}

// describeUnknown builds a rejection reason that points at skill dirs found
// under src, so "owner/repo" refs suggest the right subpath.
func describeUnknown(src string) string {
	info, err := os.Stat(src)
	if err != nil || !info.IsDir() {
		return "not a SKILL.md directory, .mdc rule, .md command or Claude plugin"
	}
	var candidates []string
	_ = filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil || len(candidates) >= 8 {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if !d.IsDir() && d.Name() == skillfs.SkillFile && filepath.Dir(p) != src {
			rel, _ := filepath.Rel(src, filepath.Dir(p))
			candidates = append(candidates, filepath.ToSlash(rel))
		}
		return nil
	})
	if len(candidates) == 0 {
		return "no SKILL.md, .claude-plugin/plugin.json or convertible file found"
	}
	return "no SKILL.md at the root; skill directories found at: " + strings.Join(candidates, ", ")
}

// convertSkillDir copies a spec skill and normalizes its frontmatter when
// it is incomplete. Valid skills pass through untouched.
func convertSkillDir(src, dstRoot, name string) (domain.ConvertedSkill, error) {
	if name == "" {
		name = filepath.Base(src)
	}
	name, err := normalizeName(name, domain.ShapeSkill, src)
	if err != nil {
		return domain.ConvertedSkill{}, err
	}
	content, err := os.ReadFile(filepath.Join(src, skillfs.SkillFile))
	if err != nil {
		return domain.ConvertedSkill{}, err
	}
	sk := domain.ConvertedSkill{Name: name, Path: filepath.Join(dstRoot, name)}
	normalized, warnings, err := normalize(content, name, nil, domain.ShapeSkill, src)
	if err != nil {
		return sk, err
	}
	sk.Warnings = warnings
	if err := skillfs.CopyTree(src, sk.Path); err != nil {
		return sk, err
	}
	if !bytes.Equal(normalized, content) {
		if err := os.WriteFile(filepath.Join(sk.Path, skillfs.SkillFile), normalized, 0o644); err != nil {
			return sk, err
		}
	}
	return sk, nil
}

// unpackPlugin converts every skills/<name> directory of a Claude plugin.
func unpackPlugin(src, dstRoot string) ([]domain.ConvertedSkill, error) {
	skillsDir := filepath.Join(src, "skills")
	found, err := skillfs.DiscoverSkills(skillsDir)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, d := range found {
		seen[d.Path] = true
	}
	for _, dir := range pluginSkillDirs(src) {
		if seen[dir] {
			continue
		}
		seen[dir] = true
		found = append(found, skillfs.Discovered{Name: filepath.Base(dir), Path: dir})
	}
	if len(found) == 0 {
		return nil, reject(domain.ShapeClaudePlugin, src, "plugin has no skills/<name>/SKILL.md")
	}
	pluginName := readPluginName(filepath.Join(src, ".claude-plugin", "plugin.json"))
	var out []domain.ConvertedSkill
	for _, d := range found {
		sk, err := convertSkillDir(d.Path, dstRoot, "")
		if err != nil {
			return out, err
		}
		sk.ConvertedFrom = string(domain.ShapeClaudePlugin)
		if pluginName != "" {
			sk.ConvertedFrom += ":" + pluginName
		}
		out = append(out, sk)
	}
	return out, nil
}

// pluginSkillDirs returns the existing skill directories listed in the
// plugin.json "skills" array (paths relative to the plugin root, used by
// plugins that keep skills outside skills/).
func pluginSkillDirs(src string) []string {
	b, err := os.ReadFile(filepath.Join(src, ".claude-plugin", "plugin.json"))
	if err != nil {
		return nil
	}
	var meta struct {
		Skills []string `json:"skills"`
	}
	if json.Unmarshal(b, &meta) != nil {
		return nil
	}
	var out []string
	for _, rel := range meta.Skills {
		dir := filepath.Join(src, filepath.FromSlash(strings.TrimPrefix(rel, "./")))
		if fileExists(filepath.Join(dir, skillfs.SkillFile)) {
			out = append(out, dir)
		}
	}
	return out
}

func readPluginName(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	var meta struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(b, &meta) != nil {
		return ""
	}
	return meta.Name
}

// convertFile turns a single-file rule or command into a skill directory.
func convertFile(shape domain.SourceShape, src, dstRoot, name string) (domain.ConvertedSkill, error) {
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	}
	name, err := normalizeName(name, shape, src)
	if err != nil {
		return domain.ConvertedSkill{}, err
	}
	content, err := os.ReadFile(src)
	if err != nil {
		return domain.ConvertedSkill{}, err
	}
	sk := domain.ConvertedSkill{Name: name, Path: filepath.Join(dstRoot, name), ConvertedFrom: string(shape)}
	var drop map[string]string // key -> warning
	switch shape {
	case domain.ShapeCursorRule:
		fm, _, _ := skillfs.ParseSkillMD(content)
		if always, ok := fm.Raw["alwaysApply"]; ok && isTrue(always) {
			return sk, reject(shape, src, "alwaysApply: true rules are always-on context, not a skill")
		}
		if globs, ok := fm.Raw["globs"]; ok && !isEmpty(globs) {
			return sk, reject(shape, src, "rules scoped by globs cannot be expressed as a skill")
		}
		drop = map[string]string{"globs": "", "alwaysApply": ""}
	case domain.ShapeOpenCodeCommand:
		drop = map[string]string{
			"agent":   "dropped OpenCode-only key 'agent'",
			"subtask": "dropped OpenCode-only key 'subtask'",
		}
	}
	normalized, warnings, err := normalize(content, name, drop, shape, src)
	if err != nil {
		return sk, err
	}
	sk.Warnings = warnings
	if err := os.MkdirAll(sk.Path, 0o755); err != nil {
		return sk, err
	}
	if err := os.WriteFile(filepath.Join(sk.Path, skillfs.SkillFile), normalized, 0o644); err != nil {
		return sk, err
	}
	return sk, nil
}

// normalize rewrites the frontmatter of content so name and description are
// present and first, drops the given keys, keeps every other key in its
// original order and leaves the body byte-for-byte unchanged.
func normalize(content []byte, name string, drop map[string]string, shape domain.SourceShape, src string) ([]byte, []string, error) {
	var warnings []string
	header, body, err := skillfs.SplitFrontmatter(content)
	switch {
	case err == nil:
	case errors.Is(err, skillfs.ErrNoFrontmatter):
		warnings = append(warnings, "no frontmatter; generated name and description")
	default:
		return nil, nil, reject(shape, src, err.Error())
	}
	keys, values, err := parseHeader(header)
	if err != nil {
		return nil, nil, reject(shape, src, "invalid frontmatter YAML: "+err.Error())
	}

	if old := scalar(values["name"]); old != "" && old != name {
		warnings = append(warnings, fmt.Sprintf("name %q replaced by %q", old, name))
	}
	description := scalar(values["description"])
	if description == "" {
		description = firstLine(body)
		if description == "" {
			return nil, nil, reject(shape, src, "no description and no body text to derive one from")
		}
		warnings = append(warnings, "description missing; using the first body line")
	}
	if utf8.RuneCountInString(description) > maxDescriptionLen {
		description = string([]rune(description)[:maxDescriptionLen])
		warnings = append(warnings, fmt.Sprintf("description truncated to %d characters", maxDescriptionLen))
	}

	root := &yaml.Node{Kind: yaml.MappingNode}
	add := func(k string, v *yaml.Node) {
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: k}, v)
	}
	add("name", &yaml.Node{Kind: yaml.ScalarNode, Value: name})
	add("description", &yaml.Node{Kind: yaml.ScalarNode, Value: description})
	for _, k := range keys {
		if k == "name" || k == "description" {
			continue
		}
		if msg, dropped := drop[k]; dropped {
			if msg != "" {
				warnings = append(warnings, msg)
			}
			continue
		}
		add(k, values[k])
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return nil, nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, nil, err
	}
	buf.WriteString("---\n")
	buf.WriteString(body)
	return buf.Bytes(), warnings, nil
}

// parseHeader decodes a YAML mapping keeping key order.
func parseHeader(header []byte) ([]string, map[string]*yaml.Node, error) {
	values := map[string]*yaml.Node{}
	if len(bytes.TrimSpace(header)) == 0 {
		return nil, values, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(header, &doc); err != nil {
		return nil, nil, err
	}
	if len(doc.Content) == 0 {
		return nil, values, nil
	}
	m := doc.Content[0]
	if m.Kind != yaml.MappingNode {
		return nil, nil, errors.New("frontmatter is not a mapping")
	}
	var keys []string
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i].Value
		if _, dup := values[k]; !dup {
			keys = append(keys, k)
		}
		values[k] = m.Content[i+1]
	}
	return keys, values, nil
}

func scalar(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return strings.TrimSpace(n.Value)
}

func isTrue(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "true")
	}
	return false
}

func isEmpty(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	case []any:
		return len(t) == 0
	}
	return false
}

// firstLine returns the first non-empty body line stripped of markdown
// heading markers, or "".
func firstLine(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if line != "" {
			return line
		}
	}
	return ""
}

// normalizeName lowercases and slugifies name, then validates it against
// the spec rule.
func normalizeName(name string, shape domain.SourceShape, src string) (string, error) {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = strings.NewReplacer(" ", "-", "_", "-", ".", "-").Replace(slug)
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	if !skillfs.NameRE.MatchString(slug) || utf8.RuneCountInString(slug) > 64 {
		return "", reject(shape, src, fmt.Sprintf("name %q cannot be normalized to %s", name, skillfs.NameRE))
	}
	return slug, nil
}

// CommandStub renders a Claude Code slash-command wrapper that invokes the
// skill, written to .claude/commands/<name>.md when --command is given.
func CommandStub(name, description, skillPath string) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "---\ndescription: %s\n---\n", yamlString(description))
	fmt.Fprintf(&buf, "Use the %s skill. Read %s and follow its instructions.\n\n$ARGUMENTS\n", name, filepath.ToSlash(filepath.Join(skillPath, skillfs.SkillFile)))
	return buf.Bytes()
}

func yamlString(s string) string {
	b, err := yaml.Marshal(s)
	if err != nil {
		return `""`
	}
	return strings.TrimSpace(string(b))
}

// SortedKeys returns the keys of m sorted; used by tests and reports.
func SortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
