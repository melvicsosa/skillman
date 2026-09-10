package tray

import _ "embed"

// Template icons generated from assets/brand/icon.png by
// internal/tray/gen/main.go (black shape + alpha, macOS template images).
var (
	//go:embed assets/tray-template-22.png
	icon22 []byte
	//go:embed assets/tray-template-44.png
	icon44 []byte
)

// Icon22 is the 1x menu bar template icon.
func Icon22() []byte { return icon22 }

// Icon44 is the 2x (retina) menu bar template icon.
func Icon44() []byte { return icon44 }
