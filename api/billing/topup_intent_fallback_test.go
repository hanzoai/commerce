package billing

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/fx"
	"github.com/hanzoai/commerce/payment/processor"
)

type localFallbackStripe struct {
	rail
	currencies []currency.Type
	methods    map[string]bool
	emptyFor   map[string]bool
	created    []processor.LocalIntent
	txs        map[string]*processor.Transaction
}

func (s *localFallbackStripe) SupportedCurrencies() []currency.Type {
	return s.currencies
}
func (s *localFallbackStripe) SupportsCurrency(cur currency.Type) bool {
	for _, c := range s.currencies {
		if strings.EqualFold(string(c), string(cur)) {
			return true
		}
	}
	return false
}
func (s *localFallbackStripe) Methods(context.Context) map[string]bool {
	return s.methods
}
func (s *localFallbackStripe) CreateLocalIntent(_ context.Context, in processor.LocalIntent) (*processor.LocalIntentOut, error) {
	s.created = append(s.created, in)
	if s.emptyFor[strings.ToLower(in.Currency)] {
		return nil, processor.ErrNoPaymentMethods
	}
	id := fmt.Sprintf("pi_local_%d_%s", len(s.created), strings.ToLower(in.Currency))
	secret := id + "_secret"
	pmTypes := []string{"card"}
	if len(in.Only) > 0 {
		pmTypes = append([]string{}, in.Only...)
	}
	if s.txs == nil {
		s.txs = map[string]*processor.Transaction{}
	}
	s.txs[id] = &processor.Transaction{
		ID:       id,
		Amount:   currency.Cents(in.Amount),
		Currency: currency.Type(in.Currency),
		Status:   "requires_payment_method",
		Metadata: map[string]interface{}{"client_secret": secret, "payment_method_types": pmTypes},
	}
	return &processor.LocalIntentOut{ID: id, ClientSecret: secret}, nil
}
func (s *localFallbackStripe) GetTransaction(_ context.Context, id string) (*processor.Transaction, error) {
	if tx, ok := s.txs[id]; ok {
		return tx, nil
	}
	return nil, processor.ErrTransactionNotFound
}

func useLocalFallbackStripe(t *testing.T, s *localFallbackStripe) {
	t.Helper()
	if s.rail.kind == "" {
		s.rail = newRail(processor.Stripe, true)
	}
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.SetConfig(&processor.RegistryConfig{
			ProcessorPriority:  []processor.ProcessorType{processor.Square, processor.Stripe},
			DisabledProcessors: map[processor.ProcessorType]bool{},
		})
		reg.Register(s)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
}

func TestOpenTopupIntentFallsBackToUSDWhenLocalCurrencyHasNoPaymentMethods(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	oldRate := fx.Rate
	fx.Rate = func(_ context.Context, cur string) (float64, error) {
		if strings.EqualFold(cur, "inr") {
			return 93.52, nil
		}
		return 1, nil
	}
	t.Cleanup(func() { fx.Rate = oldRate })

	st := &localFallbackStripe{
		currencies: []currency.Type{currency.USD, currency.INR},
		methods:    map[string]bool{},
		emptyFor:   map[string]bool{"inr": true},
	}
	useLocalFallbackStripe(t, st)

	out, f := OpenTopupIntent(ctx, org, TopupIntentIn{
		Subject:     "wallet-evm-inr-test",
		Country:     "IN",
		AmountCents: 5000,
	})
	if f != nil {
		t.Fatalf("OpenTopupIntent failed instead of falling back to USD: %d %s (%v)", f.Status, f.Message, f.Err)
	}
	if len(st.created) != 2 || st.created[0].Currency != "inr" || st.created[1].Currency != "usd" {
		t.Fatalf("expected INR attempt followed by USD fallback, got %+v", st.created)
	}
	if out.Currency != "usd" || out.Amount != 5000 || out.UsdCents != 5000 || out.Quote.Currency != "usd" {
		t.Fatalf("fallback output = %+v, want $50.00 USD", out)
	}
	rec, found, _ := pendingintent.Get(out.IntentID)
	if !found || rec.Cents != 5000 || rec.PaidCurrency != "" || rec.PaidCents != 0 {
		t.Fatalf("fallback pendingintent = %+v, want clean USD record", rec)
	}
}

func TestOpenTopupIntentReplaysOpenACHIntentWithoutDuplicates(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	st := &localFallbackStripe{
		currencies: []currency.Type{currency.USD},
		methods:    map[string]bool{"us_bank_account": true},
	}
	useLocalFallbackStripe(t, st)

	// Open an all-methods USD intent first.
	card1, f := OpenTopupIntent(ctx, org, TopupIntentIn{Subject: "ach-replay-buyer", Country: "DE", Currency: "usd", AmountCents: 5000})
	if f != nil {
		t.Fatalf("card open: %+v", f)
	}
	// Opening ACH must NOT replay the all-methods intent.
	ach1, f := OpenTopupIntent(ctx, org, TopupIntentIn{Subject: "ach-replay-buyer", Country: "DE", Currency: "usd", Method: "ach", AmountCents: 5000})
	if f != nil {
		t.Fatalf("ach1 open: %+v", f)
	}
	if ach1.IntentID == card1.IntentID {
		t.Fatalf("ACH replayed all-methods intent %s", card1.IntentID)
	}
	// Reloading ACH tab for the same amount MUST replay ach1 instead of minting a duplicate.
	ach2, f := OpenTopupIntent(ctx, org, TopupIntentIn{Subject: "ach-replay-buyer", Country: "DE", Currency: "usd", Method: "ach", AmountCents: 5000})
	if f != nil {
		t.Fatalf("ach2 open: %+v", f)
	}
	if ach2.IntentID != ach1.IntentID {
		t.Fatalf("ACH reload minted duplicate %s instead of replaying %s", ach2.IntentID, ach1.IntentID)
	}
	// All-methods reload MUST replay card1, never ach1.
	card2, f := OpenTopupIntent(ctx, org, TopupIntentIn{Subject: "ach-replay-buyer", Country: "DE", Currency: "usd", AmountCents: 5000})
	if f != nil {
		t.Fatalf("card2 open: %+v", f)
	}
	if card2.IntentID != card1.IntentID {
		t.Fatalf("card reload got %s, want %s", card2.IntentID, card1.IntentID)
	}
	if len(st.created) != 2 {
		t.Fatalf("expected exactly 2 Stripe intents (1 all-methods + 1 ACH), created %d", len(st.created))
	}
}
