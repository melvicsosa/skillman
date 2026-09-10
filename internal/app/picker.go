package app

import (
	"context"

	"github.com/melvicsosa/skillman/internal/domain"
)

// pickFolderPrompt is the message shown in the native folder dialog.
const pickFolderPrompt = "Choose a project folder"

// PickFolder opens the native folder dialog and returns the chosen absolute
// path. It returns domain.ErrCanceled when the user dismisses the dialog and
// domain.ErrUnsupported when no picker is wired for this platform.
func (s *Service) PickFolder(ctx context.Context) (string, error) {
	if s.picker == nil {
		return "", domain.ErrUnsupported
	}
	return s.picker.PickFolder(ctx, pickFolderPrompt)
}
