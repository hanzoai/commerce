// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package commerce

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	luxlog "github.com/luxfi/log"

	"github.com/zap-proto/zip"
)

// TestMount_EquivalentToStandalone proves the cloud mount and the standalone
// boot serve the SAME response on /v1/commerce/*.
//
// It used to compare a gin engine against that same engine fronted by
// zip.AdaptNetHTTP — a test that the adapter was transparent. Both halves of
// that premise are gone: v1.48.0 removed gin from the serving path, and the
// mount now registers commerce's handlers directly on the host's zip.App. So
// the question is no longer "does the adapter distort the response" but the one
// that still matters and always did: does a request get a different answer
// depending on whether commerce is hosted or standalone.
//
// Both sides are native zip. The standalone side builds its own app; the cloud
// side registers onto a host app. Same handlers, same middleware chain, so any
// divergence is a real routing or resolution difference — which is exactly what
// this is here to catch.
//
// The /healthz asymmetry is by design (HIP-0106): cloud surfaces are scoped to
// /v1/commerce + /_/commerce so one binary can host iam, base, kms and gateway
// side by side without root-path collisions. Probes use /_/commerce/healthz.
//
// It asks that of REGISTERED routes. It used to ask it of /v1/commerce/tenant
// and /v1/commerce/health, neither of which has been a route since org-as-tenant
// became GET /v1/commerce/org — so both modes 404'd and the test was comparing
// which catch-all rendered the 404, not what commerce answers. Standalone has a
// SPA fallback and the mount deliberately has none (the host owns unmatched
// paths, or commerce would swallow every other subsystem's misses), so those two
// bodies can never match and the comparison could only ever fail. Status parity
// on a miss is the invariant that survives; body parity belongs to routes that
// exist.
func TestMount_EquivalentToStandalone(t *testing.T) {
	tmp := t.TempDir()

	// Standalone boot — commerce builds and owns its own app.
	standalone, err := Embed(context.Background(), EmbedConfig{
		DataDir:  filepath.Join(tmp, "standalone"),
		HTTPAddr: "127.0.0.1:0",
		Dev:      true,
	})
	if err != nil {
		t.Fatalf("standalone Embed: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = standalone.Stop(ctx)
	})

	// Cloud boot — a host zip.App with commerce mounted onto it.
	zapp := zip.New(zip.Config{Logger: luxlog.New("test")})
	if err := Mount(zapp, filepath.Join(tmp, "cloud"), luxlog.New("test")); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	cases := []struct {
		name string
		path string
		host string
		// body says whether the two modes must agree on the bytes as well as
		// the status. False for a path neither mode registers: the answer then
		// comes from whoever owns unmatched paths, which is commerce standalone
		// and the HOST when mounted — different renderers by design.
		body bool
	}{
		// The org itself, resolved through the same host->brand->org resolver.
		{name: "org_resolved", path: "/v1/commerce/org", host: "pay.example.test", body: true},
		// The public catalog projection, same handler both modes.
		{name: "catalog", path: "/v1/commerce/catalog", host: "pay.example.test", body: true},
		// Nothing registers this. Both must still MISS — a path that resolves in
		// one mode and not the other is the drift worth catching.
		{name: "unregistered", path: "/v1/commerce/health", host: "pay.example.test"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reqStandalone := httptest.NewRequest(http.MethodGet, tc.path, nil)
			reqStandalone.Host = tc.host
			respStandalone, err := standalone.Zip().Test(reqStandalone)
			if err != nil {
				t.Fatalf("standalone Fiber Test: %v", err)
			}
			standaloneBody, _ := io.ReadAll(respStandalone.Body)

			reqCloud := httptest.NewRequest(http.MethodGet, tc.path, nil)
			reqCloud.Host = tc.host
			respCloud, err := zapp.Test(reqCloud)
			if err != nil {
				t.Fatalf("cloud Fiber Test: %v", err)
			}
			cloudBody, _ := io.ReadAll(respCloud.Body)

			if respStandalone.StatusCode != respCloud.StatusCode {
				t.Fatalf("status drift on %s: standalone=%d cloud=%d standaloneBody=%q cloudBody=%q",
					tc.path, respStandalone.StatusCode, respCloud.StatusCode, string(standaloneBody), string(cloudBody))
			}
			if tc.body && !bytes.Equal(standaloneBody, cloudBody) {
				t.Fatalf("body drift on %s:\n  standalone: %q\n  cloud:      %q", tc.path, string(standaloneBody), string(cloudBody))
			}
		})
	}
}

// The mount serves its own health probe natively, on the commerce-scoped path.
// A probe that targeted root /healthz would answer for the whole binary, not
// for this subsystem.
func TestMount_ServesScopedHealthz(t *testing.T) {
	zapp := zip.New(zip.Config{Logger: luxlog.New("test")})
	if err := Mount(zapp, t.TempDir(), luxlog.New("test")); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	resp, err := zapp.Test(httptest.NewRequest(http.MethodGet, "/_/commerce/healthz", nil))
	if err != nil {
		t.Fatalf("Fiber Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/_/commerce/healthz = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte(`"service":"commerce"`)) {
		t.Fatalf("healthz body = %q, want it to name the subsystem", string(body))
	}
}
