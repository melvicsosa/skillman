package http

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"time"

	"github.com/melvicsosa/skillman/internal/domain"
)

// pickFolderTimeout bounds how long the native dialog may stay open.
const pickFolderTimeout = 5 * time.Minute

// handlePickFolder opens the native folder dialog on the machine running the
// server. It requires a JSON content type: a cross-site HTML form can only
// send "simple" content types, and a cross-origin JSON request needs a CORS
// preflight this server never approves, so other sites cannot pop a dialog.
//
//	200 {"path": "..."}  folder chosen
//	204                  user canceled
//	415                  content type is not application/json
//	501                  no native picker on this platform
func (s *Server) handlePickFolder(w http.ResponseWriter, r *http.Request) {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		writeErrorStatus(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), pickFolderTimeout)
	defer cancel()
	path, err := s.svc.PickFolder(ctx)
	if errors.Is(err, domain.ErrCanceled) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path})
}
