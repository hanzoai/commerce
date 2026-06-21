package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/payment/processor"
	squareprovider "github.com/hanzoai/commerce/payment/providers/square"
)

// squareSigHeader is the exact header Square sends with each webhook
// delivery; lower-case on the wire, but http.Header canonicalises it.
const squareSigHeader = "x-square-hmacsha256-signature"

// squareSign reproduces the Square HMAC-SHA256(base64) signature so tests
// can drive the validated (happy) path without a live Square account. It is
// the same computation thirdparty/square/processor.go::ValidateWebhook
// verifies against, so a matching signature must validate.
func squareSign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// ─── pickSignatureHeader ────────────────────────────────────────────────

func TestPickSignatureHeader(t *testing.T) {
	cases := []struct {
		name         string
		providerHint string
		headers      map[string]string
		want         string
	}{
		{
			name:         "square hint picks square header",
			providerHint: "square",
			headers:      map[string]string{squareSigHeader: "sq-sig-value"},
			want:         "sq-sig-value",
		},
		{
			name:         "square header found via candidate list without hint",
			providerHint: "",
			headers:      map[string]string{squareSigHeader: "sq-sig-value"},
			want:         "sq-sig-value",
		},
		{
			name:         "square header found even with mismatched hint",
			providerHint: "paypal",
			headers:      map[string]string{squareSigHeader: "sq-sig-value"},
			want:         "sq-sig-value",
		},
		{
			name:         "stripe hint still works (no regression)",
			providerHint: "stripe",
			headers:      map[string]string{"Stripe-Signature": "t=1,v1=abc"},
			want:         "t=1,v1=abc",
		},
		{
			name:         "no recognised header returns empty",
			providerHint: "square",
			headers:      map[string]string{"X-Unknown": "nope"},
			want:         "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			for k, v := range tc.headers {
				h.Set(k, v)
			}
			if got := pickSignatureHeader(h, tc.providerHint); got != tc.want {
				t.Errorf("pickSignatureHeader(%q) = %q, want %q", tc.providerHint, got, tc.want)
			}
		})
	}
}

// ─── registry population ────────────────────────────────────────────────

// TestGlobalRegistryPopulated proves the blank-import of the provider barrel
// in webhooks.go actually registers providers with processor.Global(). Before
// the fix the global registry was empty (no importer of the barrel), so this
// would fail with "processor square not registered".
func TestGlobalRegistryPopulated(t *testing.T) {
	reg := processor.Global()

	if len(reg.ListTypes()) == 0 {
		t.Fatal("global processor registry is empty; provider barrel not imported")
	}

	if _, err := reg.Get(processor.Square); err != nil {
		t.Fatalf("Square processor not in global registry: %v", err)
	}

	// Stripe is the default fiat processor and must also be present.
	if _, err := reg.Get(processor.Stripe); err != nil {
		t.Fatalf("Stripe processor not in global registry: %v", err)
	}
}

// ─── HandleProviderWebhook end-to-end ───────────────────────────────────

// newTestEngine builds a gin engine with the real webhook route mounted so
// tests exercise routing + handler exactly as production does.
func newTestEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/v1/billing/webhooks/:provider", HandleProviderWebhook)
	return r
}

func TestHandleProviderWebhook_MissingSignature(t *testing.T) {
	r := newTestEngine()

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhooks/square", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for missing signature header", w.Code)
	}
}

// TestHandleProviderWebhook_BadSignatureReachesSquare proves the dispatcher
// now reaches Square's ValidateWebhook: a recognised Square signature header
// is present (fix 2) and a Square processor is registered (fix 1), so the
// request gets PAST header selection and processor lookup and is rejected by
// Square's HMAC comparison — a 401, not a 400-missing-header or a silent
// "no processor" miss.
func TestHandleProviderWebhook_BadSignatureReachesSquare(t *testing.T) {
	// Register a configured Square processor so the fast path resolves to a
	// processor whose ValidateWebhook actually runs the HMAC comparison.
	prev, _ := processor.Global().Get(processor.Square)
	t.Cleanup(func() {
		if prev != nil {
			processor.Global().Register(prev)
		}
	})
	processor.Global().Register(squareprovider.NewProvider(squareprovider.Config{
		AccessToken:         "test-token",
		LocationID:          "L1",
		WebhookSignatureKey: "whsec_test",
		Environment:         "sandbox",
	}))

	r := newTestEngine()

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhooks/square", strings.NewReader(`{"type":"payment.created"}`))
	req.Header.Set(squareSigHeader, "this-is-not-a-valid-signature")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (signature rejected by Square)", w.Code)
	}

	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v (raw=%s)", err, w.Body.String())
	}
	if !strings.Contains(body.Error.Message, "invalid webhook signature") {
		t.Errorf("error message = %q, want it to mention invalid webhook signature", body.Error.Message)
	}
}

// TestHandleProviderWebhook_ValidSignaturePassesValidation proves the full
// previously-dead path is live: a correctly-signed Square webhook clears
// Square's HMAC ValidateWebhook and advances to the next stage. The test
// carries no organization context, so the handler stops at the org-context
// gate (503) — which can ONLY be reached after signature validation
// succeeded. A 401 here would mean validation was still unreachable.
func TestHandleProviderWebhook_ValidSignaturePassesValidation(t *testing.T) {
	const secret = "whsec_valid_path"

	prev, _ := processor.Global().Get(processor.Square)
	t.Cleanup(func() {
		if prev != nil {
			processor.Global().Register(prev)
		}
	})
	processor.Global().Register(squareprovider.NewProvider(squareprovider.Config{
		AccessToken:         "test-token",
		LocationID:          "L1",
		WebhookSignatureKey: secret,
		Environment:         "sandbox",
	}))

	body := []byte(`{"type":"payment.created","event_id":"evt_1","data":{"object":{"id":"obj_1"}}}`)
	sig := squareSign(secret, body)

	r := newTestEngine()
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhooks/square", strings.NewReader(string(body)))
	req.Header.Set(squareSigHeader, sig)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusUnauthorized || w.Code == http.StatusBadRequest {
		t.Fatalf("status = %d: signature validation should have SUCCEEDED and advanced past validation, but it did not", w.Code)
	}
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (validation passed, stopped at missing org context)", w.Code)
	}
}
