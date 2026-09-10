package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/melvicsosa/skillman/internal/domain"
)

func TestRawFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github.raw+json" {
			t.Errorf("accept = %q", r.Header.Get("Accept"))
		}
		switch r.URL.Path {
		case "/repos/o/r/contents/.claude-plugin/marketplace.json":
			if r.URL.Query().Get("ref") != "dev" {
				t.Errorf("ref = %q", r.URL.Query().Get("ref"))
			}
			w.Write([]byte(`{"plugins":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"Not Found"}`))
		}
	}))
	defer srv.Close()
	c := New("")
	c.BaseURL = srv.URL
	b, err := c.RawFile(context.Background(), "o", "r", "dev", ".claude-plugin/marketplace.json")
	if err != nil || string(b) != `{"plugins":[]}` {
		t.Fatalf("RawFile = %q, %v", b, err)
	}
	if _, err := c.RawFile(context.Background(), "o", "r", "", "missing.json"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing err = %v", err)
	}
}
