// Package http exposes the JSON API and serves the embedded web UI.
package http

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/melvicsosa/skillman/internal/adapters/agents"
	"github.com/melvicsosa/skillman/internal/app/version"
	"github.com/melvicsosa/skillman/web"
)

// agentView is the JSON shape returned by GET /api/agents.
type agentView struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	GlobalDirs  []string `json:"globalDirs"`
	ProjectDirs []string `json:"projectDirs"`
	Exists      bool     `json:"exists"`
}

// NewHandler builds the root http.Handler: API routes plus the SPA.
func NewHandler() (http.Handler, error) {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("GET /api/agents", handleAgents)
	mux.Handle("/", spaHandler(dist))
	return mux, nil
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": version.Version,
		"commit":  version.Commit,
		"date":    version.Date,
	})
}

func handleAgents(w http.ResponseWriter, _ *http.Request) {
	out := make([]agentView, 0, len(agents.Registry))
	for _, s := range agents.Registry {
		out = append(out, agentView{
			ID:          string(s.ID),
			Name:        s.Name,
			GlobalDirs:  s.ResolvedGlobalDirs(),
			ProjectDirs: append([]string{}, s.ProjectDirs...),
			Exists:      s.Exists(),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// spaHandler serves static files from dist and falls back to index.html for
// any unknown non-API path so client-side routing works.
func spaHandler(dist fs.FS) http.Handler {
	fileServer := http.FileServerFS(dist)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		if info, err := fs.Stat(dist, p); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback.
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
