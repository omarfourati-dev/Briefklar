// Package static embeds the built web files: landing page at the root, Angular app under app/.
// The Docker build fills static/dist; in a plain checkout only .gitkeep is there.
package static

import "embed"

//go:embed all:dist
var Files embed.FS
