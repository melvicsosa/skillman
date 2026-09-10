package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeUsage(t *testing.T) {
	home := t.TempDir()
	t.Setenv(HomeEnv, home)

	got, p, err := (ClaudeUsage{}).Usage()
	if err != nil || len(got) != 0 || p != filepath.Join(home, ".claude.json") {
		t.Fatalf("missing file: got %v, %q, %v", got, p, err)
	}

	body := `{"skillUsage":{"branch-pr":{"usageCount":10,"lastUsedAt":1788913317724},"vercel:deploy":{"lastUsedAt":0,"usageCount":2}},"other":1}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, _, err = (ClaudeUsage{}).Usage()
	if err != nil {
		t.Fatal(err)
	}
	if got["branch-pr"].UsageCount != 10 || got["branch-pr"].LastUsedAt.Year() != 2026 {
		t.Fatalf("branch-pr = %+v", got["branch-pr"])
	}
	if !got["vercel:deploy"].LastUsedAt.IsZero() || got["vercel:deploy"].UsageCount != 2 {
		t.Fatalf("vercel:deploy = %+v", got["vercel:deploy"])
	}

	if err := os.WriteFile(p, []byte("{nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := (ClaudeUsage{}).Usage(); err == nil {
		t.Fatal("expected parse error")
	}
}
