// Package web serves the embedded single-page app.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// static holds the built SPA. The Dockerfile copies the Vite output here; in a
// plain checkout it only contains .gitkeep and Handler answers 404.
//
//go:embed all:static
var static embed.FS

// Handler serves the embedded SPA.
func Handler() http.Handler {
	sub, _ := fs.Sub(static, "static")
	return handler(sub)
}

// handler serves files from fsys, falling back to index.html for client-side
// routes. Paths with a file extension that don't exist are a real 404.
func handler(fsys fs.FS) http.Handler {
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if info, err := fs.Stat(fsys, name); err == nil && !info.IsDir() {
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}
		if path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
		index, err := fs.ReadFile(fsys, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(index)
	})
}
