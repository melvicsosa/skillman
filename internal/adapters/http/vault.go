package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/melvicsosa/skillman/internal/app"
	"github.com/melvicsosa/skillman/internal/domain"
)

// installBody is the JSON body of install and add requests.
type installBody struct {
	Ref     string   `json:"ref"`
	Agents  []string `json:"agents"`
	All     bool     `json:"all"`
	Project string   `json:"project"`
	Copy    bool     `json:"copy"`
	Command bool     `json:"command"`
}

func (b installBody) options() app.InstallOptions {
	opts := app.InstallOptions{All: b.All, Root: strings.TrimSpace(b.Project), Copy: b.Copy, Command: b.Command}
	for _, a := range b.Agents {
		opts.Agents = append(opts.Agents, domain.AgentID(a))
	}
	return opts
}

func (s *Server) handleListVault(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.VaultList(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ToVaultViews(list))
}

func (s *Server) handleRemoveVault(w http.ResponseWriter, r *http.Request) {
	force := r.URL.Query().Get("force") == "true" || r.URL.Query().Get("force") == "1"
	res, err := s.svc.VaultRemove(r.Context(), r.PathValue("name"), force)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleInstallVault(w http.ResponseWriter, r *http.Request) {
	var body installBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || (len(body.Agents) == 0 && !body.All) {
		writeErrorStatus(w, http.StatusBadRequest, `body must be {"agents": ["codex", ...] | "all": true, "project"?: "/root", "copy"?: bool, "command"?: bool}`)
		return
	}
	links, err := s.svc.InstallFromVault(r.Context(), r.PathValue("name"), body.options())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"links": nonNilLinks(links)})
}

func (s *Server) handleUninstallVault(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Agent   string `json:"agent"`
		Project string `json:"project"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Agent) == "" {
		writeErrorStatus(w, http.StatusBadRequest, `body must be {"agent": "codex", "project"?: "/root"}`)
		return
	}
	removed, err := s.svc.UninstallFromVault(r.Context(), r.PathValue("name"), domain.AgentID(body.Agent), body.Project)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": removed})
}

func (s *Server) handleUpdateVault(w http.ResponseWriter, r *http.Request) {
	res, err := s.svc.VaultUpdate(r.Context(), r.PathValue("name"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleRegistrySearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeErrorStatus(w, http.StatusBadRequest, "q is required")
		return
	}
	res, err := s.svc.Search(r.Context(), q, r.URL.Query().Get("source"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleRegistryTrending(w http.ResponseWriter, r *http.Request) {
	hits, err := s.svc.Trending(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if hits == nil {
		hits = []domain.RegistryResult{}
	}
	writeJSON(w, http.StatusOK, hits)
}

func (s *Server) handleRegistryCurated(w http.ResponseWriter, r *http.Request) {
	owners, err := s.svc.Curated(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if owners == nil {
		owners = []domain.CuratedOwner{}
	}
	writeJSON(w, http.StatusOK, owners)
}

func (s *Server) handleRegistryAdd(w http.ResponseWriter, r *http.Request) {
	var body installBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Ref) == "" {
		writeErrorStatus(w, http.StatusBadRequest, `body must be {"ref": "...", "agents"?: [...], "all"?: bool, "project"?: "/root", "copy"?: bool}`)
		return
	}
	res, err := s.svc.Add(r.Context(), strings.TrimSpace(body.Ref), body.options())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (s *Server) handleImportLock(w http.ResponseWriter, r *http.Request) {
	var body struct {
		File string `json:"file"`
	}
	if r.ContentLength != 0 {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	res, err := s.svc.ImportLock(r.Context(), body.File)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func nonNilLinks(l []app.LinkResult) []app.LinkResult {
	if l == nil {
		return []app.LinkResult{}
	}
	return l
}
