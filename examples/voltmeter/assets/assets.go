// Package assets holds the files Voltmeter serves under /assets: a
// landing page and its stylesheet, embedded in the binary so the server
// reads no disk (spec §requests-and-responses, volt.File). It lives
// apart from the routes package because an embedded file system is a
// package-level var, which the packagestate rule reads as state.
package assets

import (
	"embed"
	"io/fs"
)

//go:embed static
var static embed.FS

// FS is the embedded static directory, rooted so a path names a file in
// it directly ("voltmeter.html").
func FS() fs.FS {
	sub, err := fs.Sub(static, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
