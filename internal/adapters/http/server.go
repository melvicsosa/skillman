// Package http exposes the JSON API and serves the embedded web UI.
package http

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/melvicsosa/skillman/internal/app"
	"github.com/melvicsosa/skillman/internal/app/version"
	"github.com/melvicsosa/skillman/internal/domain"
	"github.com/melvicsosa/skillman/web"
)

// Server holds the handlers' dependencies.
type Server struct {
	svc *app.Service
}

// NewHandler builds the root http.Handler: API routes (PLAN.md section 7,
// Phase 1 subset) plus the SPA.
func NewHandler(svc *app.Service) (http.Handler, error) {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return nil, err
	}
	s := &Server{svc: svc}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("GET /api/agents", s.handleListAgents)
	mux.HandleFunc("PATCH /api/agents/{id}", s.handlePatchAgent)
	mux.HandleFunc("GET /api/skills", s.handleListSkills)
	mux.HandleFunc("POST /api/skills/{id}/enable", s.handleEnableSkill)
	mux.HandleFunc("POST /api/skills/{id}/disable", s.handleDisableSkill)
	mux.HandleFunc("GET /api/projects", s.handleListProjects)
	mux.HandleFunc("POST /api/projects", s.handleAddProject)
	mux.HandleFunc("DELETE /api/projects/{id}", s.handleRemoveProject)
	mux.HandleFunc("POST /api/scan", s.handleScan)
	mux.HandleFunc("GET /api/doctor", s.handleDoctor)
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

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListAgents(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ToAgentViews(list))
}

func (s *Server) handlePatchAgent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil {
		writeErrorStatus(w, http.StatusBadRequest, `body must be {"enabled": true|false}`)
		return
	}
	if _, err := s.svc.ToggleAgent(r.Context(), domain.AgentID(r.PathValue("id")), *body.Enabled); err != nil {
		writeError(w, err)
		return
	}
	list, err := s.svc.ListAgents(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	for _, a := range list {
		if string(a.ID) == r.PathValue("id") {
			writeJSON(w, http.StatusOK, ToAgentView(a))
			return
		}
	}
	writeErrorStatus(w, http.StatusNotFound, "agent not found")
}

func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := domain.SkillFilter{
		Agent: domain.AgentID(q.Get("agent")),
		Query: q.Get("q"),
		State: domain.SkillState(q.Get("state")),
	}
	switch scope := q.Get("scope"); {
	case q.Get("project") != "":
		f.Scope = domain.ProjectScope(q.Get("project"))
	case scope == "global":
		f.Scope = domain.GlobalScope
	case scope != "" && scope != "project":
		f.Scope = domain.Scope(scope)
	}
	skills, err := s.svc.ListSkills(r.Context(), f)
	if err != nil {
		writeError(w, err)
		return
	}
	if q.Get("scope") == "project" && q.Get("project") == "" {
		filtered := skills[:0]
		for _, sk := range skills {
			if sk.Scope.IsProject() {
				filtered = append(filtered, sk)
			}
		}
		skills = filtered
	}
	writeJSON(w, http.StatusOK, ToSkillViews(skills))
}

func (s *Server) handleEnableSkill(w http.ResponseWriter, r *http.Request) {
	sk, err := s.svc.EnableSkill(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ToSkillView(sk))
}

func (s *Server) handleDisableSkill(w http.ResponseWriter, r *http.Request) {
	sk, err := s.svc.DisableSkill(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ToSkillView(sk))
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListProjects(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ToProjectViews(list))
}

func (s *Server) handleAddProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Root string `json:"root"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Root) == "" {
		writeErrorStatus(w, http.StatusBadRequest, `body must be {"root": "/absolute/path"}`)
		return
	}
	p, sum, err := s.svc.RegisterProject(r.Context(), strings.TrimSpace(body.Root))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"project": ToProjectView(p), "scan": sum})
}

func (s *Server) handleRemoveProject(w http.ResponseWriter, r *http.Request) {
	if _, err := s.svc.RemoveProject(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	opts := app.ScanOptions{}
	if p := r.URL.Query().Get("project"); p != "" {
		opts.ProjectRoot = &p
	}
	sum, err := s.svc.Scan(r.Context(), opts)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

func (s *Server) handleDoctor(w http.ResponseWriter, r *http.Request) {
	report, err := s.svc.Doctor(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// spaHandler serves static files from dist and falls back to index.html for
// any unknown non-API path so client-side routing works.
func spaHandler(dist fs.FS) http.Handler {
	fileServer := http.FileServerFS(dist)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeErrorStatus(w, http.StatusNotFound, "no such route")
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

// writeError maps domain errors to status codes with a {"error": "..."} body.
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrReadOnly), errors.Is(err, domain.ErrAmbiguous):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrExists):
		status = http.StatusConflict
	case errors.Is(err, fs.ErrNotExist):
		status = http.StatusBadRequest
	}
	writeErrorStatus(w, status, err.Error())
}

func writeErrorStatus(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
