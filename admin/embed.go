// Package admin embeds the static commerce admin SPA into the commerce binary.
//
// The SPA is built from /Users/z/work/hanzo/commerce/app/admin via Next.js
// `output: 'export'` (static bundle in /app/admin/out). The build is copied
// into /admin/dist by the Dockerfile, then go:embed'd here and served at
// /admin/* by commerce.go's router.
package admin

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// Static is the embedded admin bundle. The actual files come from /admin/dist
// which the Dockerfile populates from the Next.js export.
//
//go:embed all:dist
var Static embed.FS

// Sub returns the dist/ subdirectory as an fs.FS, ready to wrap with
// http.FS for serving.
func Sub() fs.FS {
	sub, err := fs.Sub(Static, "dist")
	if err != nil {
		// embedding failed — return empty FS so the binary still starts.
		return embed.FS{}
	}
	return sub
}

// Handler returns an http.Handler that serves the SPA at the given prefix
// (typically "/admin"). Falls back to index.html for client-side routing.
func Handler(prefix string) http.Handler {
	root := Sub()
	fileServer := http.FileServer(http.FS(root))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip the prefix so the FS lookup matches embedded paths.
		path := strings.TrimPrefix(r.URL.Path, prefix)
		if path == "" {
			path = "/"
		}
		path = strings.TrimPrefix(path, "/")

		// Asset request (has a file extension) — serve directly with caching.
		if i := strings.LastIndexByte(path, '.'); i >= 0 && !strings.Contains(path[i:], "/") {
			if _, err := fs.Stat(root, path); err == nil {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/" + path
				fileServer.ServeHTTP(w, r2)
				return
			}
		}

		// SPA fallback — serve index.html for any unmatched route.
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		idx, err := fs.ReadFile(root, "index.html")
		if err != nil {
			http.Error(w, "admin SPA not built", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write(idx)
	})
}
