package handler

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/veerbal/opdq/internal/web"
)

func spaHandler() http.Handler {
	dist, err := fs.Sub(web.Files, "dist")
	if err != nil {
		panic(err)
	}
	files := http.FileServerFS(dist)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}

		if _, err := fs.Stat(dist, name); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}

		files.ServeHTTP(w, r)
	})
}
