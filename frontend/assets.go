// Package frontend embeds the built Vue frontend.
package frontend

import "embed"

// Dist holds dist/index.html (a committed placeholder) and dist/app (the Vite output, if built).
//
//go:embed all:dist
var Dist embed.FS
