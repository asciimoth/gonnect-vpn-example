package web

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
)

//go:embed index.html main.js wasm_exec.js app.wasm
var assets embed.FS

func Handler() http.Handler {
	mime.AddExtensionType(".wasm", "application/wasm")

	root, err := fs.Sub(assets, ".")
	if err != nil {
		panic(err)
	}

	return http.FileServer(http.FS(root))
}
