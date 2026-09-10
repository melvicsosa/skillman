package http

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/melvicsosa/skillman/internal/app"
	"github.com/melvicsosa/skillman/internal/domain"
)

// settingsView is GET /api/settings; PATCH adds restartRequired.
type settingsView struct {
	app.Settings
	DataDir         string `json:"dataDir"`
	RestartRequired bool   `json:"restartRequired,omitempty"`
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	st, err := s.svc.GetSettings(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settingsView{Settings: st, DataDir: s.svc.DataDir()})
}

// handlePatchSettings stores the patch. When the port changed and a server
// is running under the service manager, restartRequired tells the UI to
// offer POST /api/service/restart.
func (s *Server) handlePatchSettings(w http.ResponseWriter, r *http.Request) {
	var body app.SettingsPatch
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErrorStatus(w, http.StatusBadRequest, `body must be {"port": 3010, "githubToken": "...", "skillsshToken": "..."}`)
		return
	}
	st, portChanged, err := s.svc.UpdateSettings(r.Context(), body)
	if err != nil {
		writeError(w, err)
		return
	}
	view := settingsView{Settings: st, DataDir: s.svc.DataDir()}
	if portChanged {
		if info, err := s.svc.ServiceStatus(r.Context()); err == nil && (info.Running || app.UnderService()) {
			view.RestartRequired = true
		}
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleServiceStatus(w http.ResponseWriter, r *http.Request) {
	info, err := s.svc.ServiceStatus(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleServiceInstall(w http.ResponseWriter, r *http.Request) {
	info, err := s.svc.ServiceInstall(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleServiceUninstall(w http.ResponseWriter, r *http.Request) {
	info, err := s.svc.ServiceUninstall(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// handleServiceRestart answers first and asks launchd to kickstart the job
// shortly after, because the kickstart kills this very process when it is
// the service instance. Should the manager refuse while we run under the
// service, exit non-zero: KeepAlive/SuccessfulExit=false makes launchd
// relaunch us.
func (s *Server) handleServiceRestart(w http.ResponseWriter, r *http.Request) {
	info, err := s.svc.ServiceStatus(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if !info.Supported {
		writeError(w, domain.ErrUnsupported)
		return
	}
	if !info.Installed {
		writeErrorStatus(w, http.StatusConflict, "service is not installed")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"restarting": true, "port": info.Port, "url": info.URL})
	go func() {
		time.Sleep(s.restartDelay)
		if err := s.svc.ServiceRestart(newBackgroundContext()); err != nil && app.UnderService() {
			os.Exit(1)
		}
	}()
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	report, err := s.svc.Usage(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}
