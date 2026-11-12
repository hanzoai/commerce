package billing

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment/providers/stripe/stripetest"
	"github.com/hanzoai/commerce/util/test/ae"
)

// These run TakePayment through the real per-org registry
// (payment.ProcessorsForOrg) and the selection rule (processor.Registry.Fiat)
// to the real Stripe provider over HTTP, against Stripe's contract
// (stripetest). The org is a test-mode org holding the deployment's Square
// sandbox account and Stripe test keys — the configuration production will
// have, in its test kind.

type cardOrg struct {
	stripe  *stripetest.Server
	squares *atomic.Int32 // requests that reached Square's API host
	org     *organization.Organization
	ctx     ae.Context
}

func stripeContractOrg(t *testing.T) *cardOrg {
	t.Helper()
	t.Setenv("SQUARE_SANDBOX_ACCESS_TOKEN", "EAAA-contract")
	t.Setenv("SQUARE_SANDBOX_LOCATION_ID", "LCONTRACT")
	t.Setenv("SQUARE_SANDBOX_APPLICATION_ID", "sandbox-sq0idb-contract")
	t.Setenv("STRIPE_TEST_SECRET_KEY", "sk_test_contract")
	t.Setenv("STRIPE_TEST_PUBLISHABLE_KEY", "pk_test_contract")

	srv := stripetest.New(t, "sk_test_contract")
	srv.Route(t)

	// Square's API host answers only "unavailable": these tests prove which
	// processor a payment goes to, and a Square payment is the one Stripe must
	// never see. Square's own charge path is covered by its tests.
	var squares atomic.Int32
	sq := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		squares.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(sq.Close)
	next := http.DefaultTransport
	http.DefaultTransport = roundTripper(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Host, "squareupsandbox.com") || strings.HasSuffix(r.URL.Host, "squareup.com") {
			r = r.Clone(r.Context())
			r.URL.Scheme, r.URL.Host = "http", strings.TrimPrefix(sq.URL, "http://")
		}
		return next.RoundTrip(r)
	})
	t.Cleanup(func() { http.DefaultTransport = next })

	injectLedger(t, newFakeLedger())
	ctx := ae.NewContext()
	t.Cleanup(ctx.Close)
	org := organization.New(datastore.New(ctx))
	org.Name = "hanzo"
	org.MustCreate()
	return &cardOrg{stripe: srv, squares: &squares, org: org, ctx: ctx}
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// While the card rail is Square, a top-up that names no processor is Square's
// in every currency — the rail is one processor, not a currency fallback — and
// Stripe is never called for it.
func TestStripeContract_SquareRailTopupNeverReachesStripe(t *testing.T) {
	o := stripeContractOrg(t)
	if _, f := TakePayment(o.ctx, o.org, TakePaymentIn{
		SourceID: "cnon:card-nonce-ok", AmountCents: 2500, Currency: "chf", Subject: "hanzo/alice", IdempotencyKey: "chf-1",
	}); f == nil {
		t.Fatal("Square answered unavailable, so the top-up must fail")
	}
	if o.stripe.Intents() != 0 || len(o.stripe.Keys) != 0 {
		t.Fatalf("Stripe was called: intents=%d keys=%q", o.stripe.Intents(), o.stripe.Keys)
	}
}

// Square stays the default where it works: a USD top-up with no provider named
// goes to Square and Stripe sees nothing.
func TestStripeContract_SquareServedNeverReachesStripe(t *testing.T) {
	o := stripeContractOrg(t)
	if _, f := TakePayment(o.ctx, o.org, TakePaymentIn{
		SourceID: "cnon:card-nonce-ok", AmountCents: 2500, Currency: "usd", Subject: "hanzo/alice", IdempotencyKey: "usd-1",
	}); f == nil {
		t.Fatal("Square answered unavailable, so the top-up must fail")
	}
	if o.squares.Load() == 0 || o.stripe.Intents() != 0 || len(o.stripe.Keys) != 0 {
		t.Fatalf("squareRequests=%d stripeIntents=%d; want the USD charge on Square and none on Stripe",
			o.squares.Load(), o.stripe.Intents())
	}
}

// The public payment config says, per currency, whose card form takes it.
func TestStripeContract_PaymentConfig(t *testing.T) {
	o := stripeContractOrg(t)
	cfg := ReadPaymentConfig(o.ctx, o.org)
	if cfg.Cards["usd"] != "square" || cfg.Cards["eur"] != "square" || cfg.Cards["chf"] != "stripe" {
		t.Fatalf("cards = %v; want usd and eur on square, chf on stripe", cfg.Cards)
	}
	if !cfg.Stripe.Enabled || cfg.Stripe.PublishableKey != "pk_test_contract" || cfg.Stripe.Environment != "test" {
		t.Fatalf("stripe = %+v", cfg.Stripe)
	}
	if cfg.Provider != "square" || cfg.ApplicationId != "sandbox-sq0idb-contract" {
		t.Fatalf("square = %s %s", cfg.Provider, cfg.ApplicationId)
	}

	// Without Stripe keys, no currency is Stripe's and Stripe is not offered.
	t.Setenv("STRIPE_TEST_SECRET_KEY", "")
	cfg = ReadPaymentConfig(o.ctx, o.org)
	if cfg.Stripe.Enabled || cfg.Stripe.PublishableKey != "" || cfg.Cards["chf"] != "" {
		t.Fatalf("with no Stripe keys: stripe=%+v cards=%v", cfg.Stripe, cfg.Cards)
	}
}
