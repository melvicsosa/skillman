package brewpath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStable(t *testing.T) {
	prefix := t.TempDir()
	cellarBin := filepath.Join(prefix, "Cellar", "skillman", "0.5.0", "bin", "skillman")
	if err := os.MkdirAll(filepath.Dir(cellarBin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cellarBin, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Stable(cellarBin); got != cellarBin {
		t.Fatalf("without opt link: got %q, want unchanged", got)
	}
	optDir := filepath.Join(prefix, "opt")
	if err := os.MkdirAll(optDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(prefix, "Cellar", "skillman", "0.5.0"), filepath.Join(optDir, "skillman")); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(optDir, "skillman", "bin", "skillman")
	if got := Stable(cellarBin); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	for _, p := range []string{"/usr/local/bin/skillman", filepath.Join(prefix, "Cellar", "skillman")} {
		if got := Stable(p); got != p {
			t.Fatalf("Stable(%q) = %q, want unchanged", p, got)
		}
	}
}
