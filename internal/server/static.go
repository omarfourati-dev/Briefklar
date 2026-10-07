package server

import (
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
)

// Angular puts a content hash into built file names (main-ABCD1234.js) – those never change and may be cached forever.
var hashed = regexp.MustCompile(`-[A-Z0-9]{8}\.(js|css)$`)

func staticHandler(files fs.FS) http.Handler {
	fileServer := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case p == "/app":
			http.Redirect(w, r, "/app/", http.StatusMovedPermanently)
			return
		case strings.HasPrefix(p, "/app/") && path.Ext(p) == "":
			// Angular routes like /app/neu: the client router takes over
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFileFS(w, r, files, "app/index.html")
			return
		case hashed.MatchString(p):
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		case p == "/" || strings.HasSuffix(p, ".html"):
			w.Header().Set("Cache-Control", "no-cache")
		}
		fileServer.ServeHTTP(w, r)
	})
}
