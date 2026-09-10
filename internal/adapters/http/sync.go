package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/melvicsosa/skillman/internal/app"
	"github.com/melvicsosa/skillman/internal/domain"
)

// handleSyncAll is POST /api/sync {dryRun?}: fan every vault entry out.
func (s *Server) handleSyncAll(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DryRun bool `json:"dryRun"`
	}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErrorStatus(w, http.StatusBadRequest, `body must be {"dryRun"?: bool}`)
			return
		}
	}
	report, err := s.svc.Sync(r.Context(), app.SyncOptions{All: true, DryRun: body.DryRun})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// handleSyncVault is POST /api/vault/{name}/sync {project?, dryRun?}.
func (s *Server) handleSyncVault(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Project string `json:"project"`
		DryRun  bool   `json:"dryRun"`
	}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErrorStatus(w, http.StatusBadRequest, `body must be {"project"?: "/root", "dryRun"?: bool}`)
			return
		}
	}
	report, err := s.svc.Sync(r.Context(), app.SyncOptions{Names: []string{r.PathValue("name")}, Root: strings.TrimSpace(body.Project), DryRun: body.DryRun})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// handlePatchVault is PATCH /api/vault/{name} {autoSync}.
func (s *Server) handlePatchVault(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AutoSync *bool `json:"autoSync"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.AutoSync == nil {
		writeErrorStatus(w, http.StatusBadRequest, `body must be {"autoSync": true|false}`)
		return
	}
	if _, err := s.svc.VaultSetAutoSync(r.Context(), r.PathValue("name"), *body.AutoSync); err != nil {
		writeError(w, err)
		return
	}
	st, err := s.svc.VaultGet(r.Context(), r.PathValue("name"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ToVaultView(st))
}

// handleRepairDrift is POST /api/doctor/drift/{name}/repair {from, to?, project?}.
func (s *Server) handleRepairDrift(w http.ResponseWriter, r *http.Request) {
	var body struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Project string   `json:"project"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.From) == "" {
		writeErrorStatus(w, http.StatusBadRequest, `body must be {"from": "vault"|"<agent>", "to"?: ["<agent>"], "project"?: "/root"}`)
		return
	}
	opts := app.RepairOptions{Name: r.PathValue("name"), From: strings.TrimSpace(body.From), Root: strings.TrimSpace(body.Project)}
	for _, a := range body.To {
		opts.To = append(opts.To, domain.AgentID(a))
	}
	res, err := s.svc.RepairDrift(r.Context(), opts)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleAdoptVault is POST /api/vault/adopt {name, from, link?}.
func (s *Server) handleAdoptVault(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		From string `json:"from"`
		Link bool   `json:"link"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" || strings.TrimSpace(body.From) == "" {
		writeErrorStatus(w, http.StatusBadRequest, `body must be {"name": "skill", "from": "<agent>", "link"?: bool}`)
		return
	}
	entry, err := s.svc.VaultAdopt(r.Context(), strings.TrimSpace(body.Name), domain.AgentID(strings.TrimSpace(body.From)), body.Link)
	if err != nil {
		writeError(w, err)
		return
	}
	st, err := s.svc.VaultGet(r.Context(), entry.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ToVaultView(st))
}

// handleExportPlugin is POST /api/vault/export {outDir, name, version?, description?, skills[]}.
func (s *Server) handleExportPlugin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OutDir      string   `json:"outDir"`
		Name        string   `json:"name"`
		Version     string   `json:"version"`
		Description string   `json:"description"`
		Skills      []string `json:"skills"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.OutDir) == "" || strings.TrimSpace(body.Name) == "" || len(body.Skills) == 0 {
		writeErrorStatus(w, http.StatusBadRequest, `body must be {"outDir": "/path", "name": "plugin", "version"?: "0.1.0", "description"?: "...", "skills": ["name", ...]}`)
		return
	}
	res, err := s.svc.ExportPlugin(r.Context(), app.ExportOptions{
		OutDir: body.OutDir, Name: body.Name, Version: body.Version, Description: body.Description, Skills: body.Skills,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}
