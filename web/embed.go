// Package web embeds the built frontend (web/dist) into the binary.
package web

import "embed"

// Dist holds the Vite build output. A placeholder index.html is committed so
// the Go build never depends on the frontend build having run.
//
//go:embed all:dist
var Dist embed.FS
