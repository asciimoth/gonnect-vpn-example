package web

import (
	"embed"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
)

//go:embed index.html main.js
var assets embed.FS

func Handler() http.Handler {
	mime.AddExtensionType(".wasm", "application/wasm")

	root, err := fs.Sub(assets, ".")
	if err != nil {
		panic(err)
	}

	embedded := http.FileServer(http.FS(root))
	mux := http.NewServeMux()
	mux.Handle("/", embedded)
	mux.Handle("/wasm_exec.js", optionalFileHandler("web/wasm_exec.js", embedded))
	mux.Handle("/app.wasm", optionalFileHandler("web/app.wasm", embedded))
	return mux
}

func optionalFileHandler(path string, fallback http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, err := os.Open(path)
		if err != nil {
			fallback.ServeHTTP(w, r)
			return
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil || info.IsDir() {
			fallback.ServeHTTP(w, r)
			return
		}

		http.ServeContent(w, r, info.Name(), info.ModTime(), readSeeker{file})
	})
}

type readSeeker struct {
	*os.File
}

func (r readSeeker) Read(p []byte) (int, error) {
	return r.File.Read(p)
}

func (r readSeeker) Seek(offset int64, whence int) (int64, error) {
	return r.File.Seek(offset, whence)
}

func (r readSeeker) Close() error {
	return r.File.Close()
}

var _ io.ReadSeeker = readSeeker{}
