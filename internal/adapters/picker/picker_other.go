//go:build !darwin

package picker

import (
	"context"

	"github.com/melvicsosa/skillman/internal/domain"
)

// PickFolder is not available outside macOS.
func (Picker) PickFolder(context.Context, string) (string, error) {
	return "", domain.ErrUnsupported
}
