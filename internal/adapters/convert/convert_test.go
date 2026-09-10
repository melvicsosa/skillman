package convert

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/domain"
)

func fixture(parts ...string) string {
	return filepath.Join(append([]string{"testdata"}, parts...)...)
}

func readSkill(t *testing.T, dir string) (skillfs.Frontmatter, string) {
	t.Helper()
	fm, body, err := skillfs.ReadSkillMD(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	return fm, body
}

func TestDetect(t *testing.T) {
	tests := []struct {
		src  string
		want domain.SourceShape
	}{
		{fixture("spec-skill"), domain.ShapeSkill},
		{fixture("incomplete-skill"), domain.ShapeSkill},
		{fixture("plugin"), domain.ShapeClaudePlugin},
		{fixture("unknown"), domain.ShapeUnknown},
		{fixture(".cursor", "rules", "convertible.mdc"), domain.ShapeCursorRule},
		{fixture(".claude", "commands", "Review_Diff.md"), domain.ShapeClaudeCommand},
		{fixture(".claude", "commands", "bare.md"), domain.ShapeClaudeCommand},
		{fixture(".opencode", "command", "plan.md"), domain.ShapeOpenCodeCommand},
		{fixture("unknown", "README.md"), domain.ShapeClaudeCommand},
	}
	for _, tt := range tests {
		got, err := Converter{}.Detect(tt.src)
		if err != nil || got != tt.want {
			t.Errorf("Detect(%s) = %s, %v; want %s", tt.src, got, err, tt.want)
		}
	}
	if _, err := (Converter{}).Detect(fixture("missing")); err == nil {
		t.Error("Detect(missing) expected error")
	}
}

func TestConvertTable(t *testing.T) {
	tests := []struct {
		name          string
		src           string
		reqName       string
		wantShape     domain.SourceShape
		wantNames     []string
		wantDesc      string
		wantFrom      string
		wantWarnings  int
		wantKeys      []string // keys that must survive
		wantNoKeys    []string
		wantBody      string
		wantReject    bool
		wantRejectHas string
	}{
		{
			name: "spec skill passthrough", src: fixture("spec-skill"), wantShape: domain.ShapeSkill,
			wantNames: []string{"spec-skill"}, wantDesc: "A valid spec skill.", wantKeys: []string{"metadata"},
			wantBody: "# Spec skill\n\nBody stays.\n",
		},
		{
			name: "incomplete skill normalized", src: fixture("incomplete-skill"), wantShape: domain.ShapeSkill,
			wantNames: []string{"incomplete-skill"}, wantDesc: "Incomplete", wantWarnings: 2, wantKeys: []string{"license"},
			wantBody: "# Incomplete\n\nFirst body line becomes the description.\n",
		},
		{
			name: "cursor rule convertible", src: fixture(".cursor", "rules", "convertible.mdc"), wantShape: domain.ShapeCursorRule,
			wantNames: []string{"convertible"}, wantDesc: "Convertible cursor rule", wantFrom: "cursor-rule",
			wantNoKeys: []string{"globs", "alwaysApply"}, wantBody: "# Rule\n\nDo the thing.\n",
		},
		{
			name: "cursor rule always apply rejected", src: fixture(".cursor", "rules", "always.mdc"),
			wantReject: true, wantRejectHas: "alwaysApply",
		},
		{
			name: "cursor rule globs rejected", src: fixture(".cursor", "rules", "globbed.mdc"),
			wantReject: true, wantRejectHas: "globs",
		},
		{
			name: "claude command", src: fixture(".claude", "commands", "Review_Diff.md"), wantShape: domain.ShapeClaudeCommand,
			wantNames: []string{"review-diff"}, wantDesc: "Review the diff", wantFrom: "claude-command",
			wantKeys: []string{"allowed-tools", "argument-hint"}, wantBody: "Review $ARGUMENTS carefully.\n",
		},
		{
			name: "claude command without frontmatter", src: fixture(".claude", "commands", "bare.md"), wantShape: domain.ShapeClaudeCommand,
			wantNames: []string{"bare"}, wantDesc: "Just a prompt with no frontmatter.", wantFrom: "claude-command", wantWarnings: 2,
			wantBody: "Just a prompt with no frontmatter.\n\nMore text.\n",
		},
		{
			name: "opencode command drops agent and subtask", src: fixture(".opencode", "command", "plan.md"), wantShape: domain.ShapeOpenCodeCommand,
			wantNames: []string{"plan"}, wantDesc: "Plan a feature", wantFrom: "opencode-command", wantWarnings: 2,
			wantKeys: []string{"model"}, wantNoKeys: []string{"agent", "subtask"}, wantBody: "Plan $ARGUMENTS.\n",
		},
		{
			name: "claude plugin unpacked", src: fixture("plugin"), wantShape: domain.ShapeClaudePlugin,
			wantNames: []string{"one", "two"}, wantFrom: "claude-plugin:demo-plugin",
		},
		{
			name: "unknown dir rejected with hints", src: fixture("unknown"),
			wantReject: true, wantRejectHas: "deep/nested-skill",
		},
		{
			name: "requested name overrides", src: fixture("spec-skill"), reqName: "My Skill", wantShape: domain.ShapeSkill,
			wantNames: []string{"my-skill"}, wantDesc: "A valid spec skill.", wantWarnings: 1,
		},
		{
			name: "bad requested name rejected", src: fixture("spec-skill"), reqName: "***",
			wantReject: true, wantRejectHas: "cannot be normalized",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := t.TempDir()
			report, err := Converter{}.Convert(tt.src, dst, tt.reqName)
			if tt.wantReject {
				var rej *RejectedError
				if !errors.Is(err, domain.ErrRejected) || !errors.As(err, &rej) || !strings.Contains(rej.Reason, tt.wantRejectHas) {
					t.Fatalf("err = %v, want rejection containing %q", err, tt.wantRejectHas)
				}
				return
			}
			if err != nil {
				t.Fatalf("Convert: %v", err)
			}
			if report.Shape != tt.wantShape || len(report.Skills) != len(tt.wantNames) {
				t.Fatalf("report = %+v", report)
			}
			for i, sk := range report.Skills {
				if sk.Name != tt.wantNames[i] || sk.Path != filepath.Join(dst, sk.Name) || sk.ConvertedFrom != tt.wantFrom {
					t.Errorf("skill[%d] = %+v", i, sk)
				}
				if len(report.Skills) == 1 && len(sk.Warnings) != tt.wantWarnings {
					t.Errorf("warnings = %v, want %d", sk.Warnings, tt.wantWarnings)
				}
				fm, body := readSkill(t, sk.Path)
				if fm.Name != sk.Name {
					t.Errorf("frontmatter name = %q", fm.Name)
				}
				if tt.wantDesc != "" && fm.Description != tt.wantDesc {
					t.Errorf("description = %q, want %q", fm.Description, tt.wantDesc)
				}
				if tt.wantBody != "" && body != tt.wantBody {
					t.Errorf("body = %q, want %q", body, tt.wantBody)
				}
				for _, k := range tt.wantKeys {
					if _, ok := fm.Raw[k]; !ok {
						t.Errorf("key %q was dropped", k)
					}
				}
				for _, k := range tt.wantNoKeys {
					if _, ok := fm.Raw[k]; ok {
						t.Errorf("key %q should have been dropped", k)
					}
				}
				if issues := skillfs.ValidateFrontmatter(fm, sk.Name); len(issues) != 0 {
					t.Errorf("converted skill has spec issues: %v", issues)
				}
			}
		})
	}
}

func TestSpecSkillPassthroughKeepsBytesAndFiles(t *testing.T) {
	dst := t.TempDir()
	if _, err := (Converter{}).Convert(fixture("spec-skill"), dst, ""); err != nil {
		t.Fatal(err)
	}
	want, _ := os.ReadFile(fixture("spec-skill", "SKILL.md"))
	got, _ := os.ReadFile(filepath.Join(dst, "spec-skill", "SKILL.md"))
	if string(got) != string(want) {
		t.Fatalf("valid SKILL.md was rewritten:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(dst, "spec-skill", "scripts", "run.sh")); err != nil {
		t.Fatal("sibling files not copied")
	}
}

func TestCommandStub(t *testing.T) {
	stub := string(CommandStub("alpha", "Does: things", "/vault/alpha"))
	if !strings.HasPrefix(stub, "---\ndescription: 'Does: things'\n---\n") || !strings.Contains(stub, "/vault/alpha/SKILL.md") || !strings.Contains(stub, "$ARGUMENTS") {
		t.Fatalf("stub = %q", stub)
	}
}
