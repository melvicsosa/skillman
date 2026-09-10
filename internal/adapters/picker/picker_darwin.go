//go:build darwin

package picker

import "context"

// PickFolder shows the macOS "choose folder" dialog via osascript.
func (p Picker) PickFolder(ctx context.Context, prompt string) (string, error) {
	return pickWithOsascript(ctx, p.run, prompt)
}
