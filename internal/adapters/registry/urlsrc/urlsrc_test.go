package urlsrc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/melvicsosa/skillman/internal/adapters/registry/archive/archivetest"
	"github.com/melvicsosa/skillman/internal/domain"
)

func TestFetchArchiveAndSkillMD(t *testing.T) {
	tgz := archivetest.TarGz(t, map[string]string{"gamma/SKILL.md": "---\nname: gamma\ndescription: d\n---\n"})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /dl/gamma.tar.gz", func(w http.ResponseWriter, _ *http.Request) { w.Write(tgz) })
	mux.HandleFunc("GET /raw/delta/SKILL.md", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("---\nname: delta\ndescription: d\n---\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	if !Looks(srv.URL+"/dl/gamma.tar.gz") || !Looks(srv.URL+"/raw/delta/SKILL.md") || Looks(srv.URL+"/page.html") || Looks("owner/repo") {
		t.Fatal("Looks misclassified")
	}
	c := New()
	ctx := context.Background()
	res, err := c.Fetch(ctx, srv.URL+"/dl/gamma.tar.gz")
	if err != nil || res.Name != "gamma" || res.Source.Type != domain.SourceURL {
		t.Fatalf("archive = %+v err=%v", res, err)
	}
	if _, err := os.Stat(filepath.Join(res.Dir, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	res.Cleanup()

	res, err = c.Fetch(ctx, srv.URL+"/raw/delta/SKILL.md")
	if err != nil || res.Name != "delta" {
		t.Fatalf("skill.md = %+v err=%v", res, err)
	}
	res.Cleanup()

	if _, err := c.Fetch(ctx, srv.URL+"/dl/missing.zip"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing err = %v", err)
	}
	if _, err := c.Fetch(ctx, srv.URL+"/page.html"); err == nil {
		t.Fatal("html url accepted")
	}
}
