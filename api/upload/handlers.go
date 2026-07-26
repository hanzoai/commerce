// Package upload is the admin surface for merchant asset uploads (product
// images). A merchant POSTs multipart file(s); each is content-sniffed (image
// bytes only — the client Content-Type header is never trusted), size-capped,
// and stored under a per-tenant prefix in the house S3 client
// (github.com/hanzoai/s3-go). The response is the public URL(s).
//
// Tenant isolation: every object key is prefixed tenant/<org>/uploads/ from the
// AUTHENTICATED org (middleware.GetOrganization) — a caller can only ever write
// under its own tenant. Admin-gated INSIDE the handler (middleware.RequireAdmin)
// because the route-level TokenRequired(Admin) middleware no-ops on the IAM path
// (Red HIGH-4) — a handler must never trust that gate alone.
package upload

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"path"

	"github.com/google/uuid"
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/infra"
	"github.com/hanzoai/commerce/middleware"
	httpjson "github.com/hanzoai/commerce/util/json/http"
)

// MaxUploadBytes caps a single upload at 10 MiB — large enough for a
// high-resolution product photo, small enough to bound memory + storage abuse.
const MaxUploadBytes = 10 << 20

// ErrNoStorage is returned when the object store is not configured for this
// deployment (no S3_URL / S3_ENDPOINT). Uploads 503 rather than silently drop.
var ErrNoStorage = errors.New("upload: object storage not configured")

// allowedExt maps a byte-sniffed image content type to its canonical file
// extension. The set is the raster formats a storefront serves; the type is
// determined by net/http.DetectContentType on the file bytes, never the
// client-supplied Content-Type, so a mislabeled or hostile upload is rejected.
// SVG is deliberately excluded (it is executable markup — a stored-XSS vector).
var allowedExt = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// Storer is the object-store surface the handler needs — satisfied by
// *infra.StorageClient. An interface (not the concrete client) so tests inject a
// fake and run with no real S3.
type Storer interface {
	Upload(ctx context.Context, opts *infra.UploadOptions) (*infra.UploadResult, error)
}

// storage is the process-wide object store, injected at bootstrap by
// SetStorage. nil ⇒ uploads 503 (ErrNoStorage).
var storage Storer

// SetStorage wires the object store (called once at bootstrap after the infra
// manager connects). Passing a nil client is a no-op — the handler stays 503.
func SetStorage(s Storer) {
	if s == nil {
		return
	}
	storage = s
}

// Route mounts the admin upload surface. args are the route-level guards
// (adminRequired) shared with the rest of the /v1 admin bundle; the handler
// re-checks admin internally.
func Route(router zip.Router, args ...zip.Handler) {
	namespaced := middleware.Namespace()
	api := router.Group("upload")

	handler := append(append([]zip.Handler{}, args...), namespaced, Images)
	api.Post("/images", handler...)
	api.Post("/image", handler...)
	api.Post("", handler...)
}

type uploadResponse struct {
	// URL is the first uploaded file's public URL (the common single-file case).
	URL string `json:"url"`
	// URLs is every uploaded file's public URL, in request order.
	URLs []string `json:"urls"`
}

// Images stores one or more posted image files under the caller's tenant prefix
// and returns their public URLs. Files may arrive under the "file" or "files"
// multipart field (repeated), so both a single-file and a gallery upload work.
func Images(c *zip.Ctx) error {
	if !middleware.RequireAdmin(c) {
		return nil
	}
	if storage == nil {
		return httpjson.Fail(c, 503, "upload storage not configured", ErrNoStorage)
	}

	org := middleware.GetOrganization(c)

	form, err := c.Fiber().MultipartForm()
	if err != nil {
		return httpjson.Fail(c, 400, "invalid multipart form", err)
	}
	files := append(append([]*multipart.FileHeader{}, form.File["file"]...), form.File["files"]...)
	if len(files) == 0 {
		return httpjson.Fail(c, 400, "no files provided (field 'file' or 'files')", errors.New("no files"))
	}

	urls := make([]string, 0, len(files))
	for _, fh := range files {
		url, status, err := storeOne(c.Context(), org.Name, fh)
		if err != nil {
			return httpjson.Fail(c, status, err.Error(), err)
		}
		urls = append(urls, url)
	}

	return httpjson.Render(c, 200, uploadResponse{URL: urls[0], URLs: urls})
}

// storeOne validates and stores a single file, returning its public URL. On
// failure it returns the HTTP status to surface (413 too large, 415 wrong type,
// 500 store failure). The stored content type is the SNIFFED type, not the
// client header.
func storeOne(ctx context.Context, org string, fh *multipart.FileHeader) (string, int, error) {
	if fh.Size > MaxUploadBytes {
		return "", 413, errors.New("file exceeds 10MB limit")
	}

	f, err := fh.Open()
	if err != nil {
		return "", 400, err
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, MaxUploadBytes+1))
	if err != nil {
		return "", 400, err
	}
	if len(data) > MaxUploadBytes {
		return "", 413, errors.New("file exceeds 10MB limit")
	}

	// Sniff the content type from the bytes — the client Content-Type is never
	// trusted. Only raster image formats are accepted.
	contentType := http.DetectContentType(data)
	ext, ok := allowedExt[contentType]
	if !ok {
		return "", 415, errors.New("unsupported file type: only JPEG, PNG, GIF, WebP images are allowed")
	}

	key := path.Join("tenant", org, "uploads", uuid.NewString()+ext)

	res, err := storage.Upload(ctx, &infra.UploadOptions{
		Key:         key,
		Reader:      bytes.NewReader(data),
		Size:        int64(len(data)),
		ContentType: contentType,
	})
	if err != nil {
		return "", 500, err
	}
	return res.Location, 0, nil
}
