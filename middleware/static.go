package middleware

import (
	"io"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/config"
)

func Static(urlRoot string) zip.Handler {
	// Shave off leading /, otherwise filepath.Join will fail.
	directory := strings.TrimLeft(urlRoot, "/")
	if !filepath.IsAbs(directory) {
		directory = filepath.Join(config.RootDir, directory)
	}
	dir := http.Dir(directory)

	return func(c *zip.Ctx) error {
		if c.Method() != "GET" && c.Method() != "HEAD" {
			return c.Next()
		}

		file := strings.Replace(c.Path(), urlRoot, "", 1)

		f, err := dir.Open(file)
		if err != nil {
			return c.NoContent(404)
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil {
			return c.NoContent(500)
		}

		if fi.IsDir() {
			file = path.Join(file, "index.html")
			f, err = dir.Open(file)
			if err != nil {
				return c.NoContent(500)
			}
			defer f.Close()
			fi, err = f.Stat()
			if err != nil || fi.IsDir() {
				return c.NoContent(500)
			}
		}

		// Read the whole asset before writing so the deferred Close can't race a
		// lazily-consumed stream (fasthttp reads a body-stream after the handler
		// returns). Static assets served here are small.
		data, err := io.ReadAll(f)
		if err != nil {
			return c.NoContent(500)
		}

		if ct := mime.TypeByExtension(filepath.Ext(file)); ct != "" {
			c.SetHeader("Content-Type", ct)
		}
		return c.Bytes(200, data)
	}
}
