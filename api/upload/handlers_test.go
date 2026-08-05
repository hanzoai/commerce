package upload

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/infra"
	"github.com/hanzoai/commerce/models/organization"
)

// fakeStore records the last uploaded key + returns a deterministic public URL.
// It lets the handler run with no real S3.
type fakeStore struct {
	keys        []string
	contentType []string
	err         error
}

func (f *fakeStore) Upload(ctx context.Context, opts *infra.UploadOptions) (*infra.UploadResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.keys = append(f.keys, opts.Key)
	f.contentType = append(f.contentType, opts.ContentType)
	// Drain the reader so a real store's byte accounting is exercised.
	n, _ := io.Copy(io.Discard, opts.Reader)
	return &infra.UploadResult{
		Key:      opts.Key,
		Size:     n,
		Location: "https://cdn.example.com/" + opts.Key,
	}, nil
}

// pngBytes returns a minimal valid PNG (sniffs as image/png).
func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// multipartBody builds a multipart form with one file under the given field.
func multipartBody(t *testing.T, field, filename string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return &body, w.FormDataContentType()
}

// callUpload drives Images over a real request wired so middleware.GetOrganization
// resolves to org `ns` and the injected IAM claim authorizes (or not) the admin
// upload. Returns status + body.
func callUpload(t *testing.T, ns string, admin bool, body *bytes.Buffer, contentType string) (int, []byte) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	seed := func(c *zip.Ctx) error {
		org := &organization.Organization{}
		org.Name = ns
		c.Locals("organization", org)
		c.Locals("iam_authenticated", true)
		c.Locals("iam_claims", &auth.IAMClaims{Owner: ns, IsAdmin: admin})
		return c.Next()
	}
	app.Post("/upload/images", seed, Images)

	req := httptest.NewRequest(http.MethodPost, "/upload/images", body)
	req.Header.Set("Content-Type", contentType)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func TestUpload_Image_ReturnsOrgPrefixedURL(t *testing.T) {
	fake := &fakeStore{}
	SetStorage(fake)
	t.Cleanup(func() { storage = nil })

	body, ct := multipartBody(t, "file", "photo.png", pngBytes(t))
	code, b := callUpload(t, "acme", true, body, ct)

	if code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", code, b)
	}
	var resp uploadResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, b)
	}
	if resp.URL == "" || len(resp.URLs) != 1 {
		t.Fatalf("expected one url, got %+v", resp)
	}
	if len(fake.keys) != 1 {
		t.Fatalf("expected one stored object, got %d", len(fake.keys))
	}
	if !strings.HasPrefix(fake.keys[0], "tenant/acme/uploads/") {
		t.Fatalf("key not org-prefixed: %q", fake.keys[0])
	}
	if !strings.HasSuffix(fake.keys[0], ".png") {
		t.Fatalf("key missing sniffed extension: %q", fake.keys[0])
	}
	if fake.contentType[0] != "image/png" {
		t.Fatalf("stored content type = %q, want image/png", fake.contentType[0])
	}
	if !strings.Contains(resp.URL, "tenant/acme/uploads/") {
		t.Fatalf("returned url not the stored object: %q", resp.URL)
	}
}

func TestUpload_RejectsNonImage(t *testing.T) {
	fake := &fakeStore{}
	SetStorage(fake)
	t.Cleanup(func() { storage = nil })

	// A text/HTML payload sniffs as text/plain or text/html — not an allowed image.
	body, ct := multipartBody(t, "file", "evil.svg", []byte("<svg onload=alert(1)></svg>"))
	code, b := callUpload(t, "acme", true, body, ct)

	if code != 415 {
		t.Fatalf("status = %d, want 415; body=%s", code, b)
	}
	if len(fake.keys) != 0 {
		t.Fatalf("non-image reached storage: %v", fake.keys)
	}
}

func TestUpload_NonAdmin_403(t *testing.T) {
	fake := &fakeStore{}
	SetStorage(fake)
	t.Cleanup(func() { storage = nil })

	body, ct := multipartBody(t, "file", "photo.png", pngBytes(t))
	code, _ := callUpload(t, "acme", false, body, ct)

	if code != 403 {
		t.Fatalf("status = %d, want 403", code)
	}
	if len(fake.keys) != 0 {
		t.Fatalf("non-admin reached storage: %v", fake.keys)
	}
}

func TestUpload_NoStorage_503(t *testing.T) {
	storage = nil // no store configured

	body, ct := multipartBody(t, "file", "photo.png", pngBytes(t))
	code, _ := callUpload(t, "acme", true, body, ct)

	if code != 503 {
		t.Fatalf("status = %d, want 503", code)
	}
}
