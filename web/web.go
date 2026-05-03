// Package web embeds the server's templates and static assets.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:templates all:static
var files embed.FS

func Templates() fs.FS { return files } // pass directly; ParseFS will look for "templates/*.html"

func Static() fs.FS {
	sub, err := fs.Sub(files, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
