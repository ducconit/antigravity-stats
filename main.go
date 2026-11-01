package main

import (
	"embed"
	"io/fs"
	"net/http"

	"antigravity-stats/cmd"
)

//go:embed all:docs
var staticFS embed.FS

func main() {
	sub, err := fs.Sub(staticFS, "docs")
	if err == nil {
		cmd.SetStaticFS(http.FS(sub))
	}
	cmd.Execute()
}
