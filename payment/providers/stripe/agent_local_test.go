package stripe

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hanzoai/commerce/payment/processor"
)

type rewriteTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (r rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = r.target.Scheme
	clone.URL.Host = r.target.Host
	clone.Host = r.target.Host
	return r.base.RoundTrip(clone)
}

func newInterceptingProvider(t *testing.T, handler http.HandlerFunc) *Provider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	p := &Provider{
		BaseProcessor:  processor.NewBaseProcessor(processor.Stripe, supportedCurrencies()),
		secretKey:      "sk_test_fake",
		publishableKey: "pk_test_fake",
		client:         &http.Client{Transport: rewriteTransport{target: u, base: srv.Client().Transport}},
	}
	p.SetConfigured(true)
	return p
}

func TestCryptoConfigurationUsesAgentVersionAndSkipsErrorCache(t *testing.T) {
	var calls atomic.Int32
	var versions []string
	p := newInterceptingProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/payment_method_configurations" {
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		versions = append(versions, r.Header.Get("Stripe-Version"))
		if calls.Add(1) == 1 {
			// First call fails transiently; must NOT poison cache for 60s.
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":{"message":"upstream hiccup"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"pmc_live_default","active":true,"is_default":true,"crypto":{"available":true,"display_preference":{"value":"on"}}}]}`))
	})

	ready1 := p.StablecoinReady(context.Background())
	if ready1.On {
		t.Fatalf("expected transient error to report On=false, got %+v", ready1)
	}
	ready2 := p.StablecoinReady(context.Background())
	if !ready2.On || len(ready2.Networks) == 0 {
		t.Fatalf("expected second call after transient error to succeed without stale error cache, got %+v", ready2)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 Stripe calls, got %d", len(versions))
	}
	for i, v := range versions {
		if v != agentVersion {
			t.Errorf("call %d Stripe-Version = %q, want %q", i+1, v, agentVersion)
		}
	}
}

func TestCreateLocalIntentCancelsEmptyPaymentMethodTypesAndScopesRegionalOptions(t *testing.T) {
	var createdForm url.Values
	var cancelledPath string
	p := newInterceptingProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/payment_intents":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			createdForm = r.PostForm
			_, _ = w.Write([]byte(`{"id":"pi_inr_empty","client_secret":"pi_inr_empty_secret","status":"requires_payment_method","amount":467600,"currency":"inr","payment_method_types":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/payment_intents/pi_inr_empty/cancel":
			cancelledPath = r.URL.Path
			_, _ = w.Write([]byte(`{"id":"pi_inr_empty","status":"canceled"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})

	_, err := p.CreateLocalIntent(context.Background(), processor.LocalIntent{
		Amount:   467600,
		Currency: "inr",
		Key:      "inr-empty-test",
	})
	if !errors.Is(err, processor.ErrNoPaymentMethods) {
		t.Fatalf("CreateLocalIntent err = %v, want ErrNoPaymentMethods", err)
	}
	if cancelledPath != "/v1/payment_intents/pi_inr_empty/cancel" {
		t.Fatalf("unusable empty-method intent was not cancelled: cancelledPath=%q", cancelledPath)
	}
	for k := range createdForm {
		if strings.Contains(k, "boleto") || strings.Contains(k, "oxxo") || strings.Contains(k, "pix") {
			t.Errorf("INR intent should not send BRL/MXN payment_method_options, got %s=%v", k, createdForm[k])
		}
	}
}

func TestCreateLocalIntentSendsRegionalOptionsForMatchingCurrency(t *testing.T) {
	seen := map[string]url.Values{}
	p := newInterceptingProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		cur := r.PostForm.Get("currency")
		seen[cur] = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"pi_` + cur + `","client_secret":"sec_` + cur + `","status":"requires_payment_method","payment_method_types":["card"]}`))
	})

	for _, cur := range []string{"brl", "mxn", "usd"} {
		out, err := p.CreateLocalIntent(context.Background(), processor.LocalIntent{Amount: 500, Currency: cur})
		if err != nil || out == nil || out.ID != "pi_"+cur {
			t.Fatalf("%s: out=%+v err=%v", cur, out, err)
		}
	}
	if seen["brl"].Get("payment_method_options[pix][expires_after_seconds]") != "86400" ||
		seen["brl"].Get("payment_method_options[boleto][expires_after_days]") != "3" ||
		seen["brl"].Get("payment_method_options[oxxo][expires_after_days]") != "" {
		t.Fatalf("brl options mismatch: %v", seen["brl"])
	}
	if seen["mxn"].Get("payment_method_options[oxxo][expires_after_days]") != "3" ||
		seen["mxn"].Get("payment_method_options[pix][expires_after_seconds]") != "" {
		t.Fatalf("mxn options mismatch: %v", seen["mxn"])
	}
	if seen["usd"].Get("payment_method_options[pix][expires_after_seconds]") != "" ||
		seen["usd"].Get("payment_method_options[oxxo][expires_after_days]") != "" ||
		seen["usd"].Get("payment_method_options[boleto][expires_after_days]") != "" {
		t.Fatalf("usd should carry no regional voucher options: %v", seen["usd"])
	}
}
