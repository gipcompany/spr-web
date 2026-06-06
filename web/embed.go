// Package web embeds the built frontend (web/dist) into the spr-web binary.
//
// dist/ is a build artifact produced by `pnpm build` and is not committed
// (only a .gitkeep placeholder keeps the embed pattern valid). This is why
// `go install` is not supported: build from a release archive or run
// `make build`, which builds the frontend first.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the built frontend as a filesystem rooted at dist/.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
