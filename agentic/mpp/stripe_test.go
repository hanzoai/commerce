// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// seller is a Seller that records what it is asked to charge.
type seller struct {
	mu      sync.Mutex
	profile *Profile
	sales   []Sale
	err     error
	ref     string
}

func newSeller() *seller {
	return &seller{profile: &Profile{Network: "profile_1MqDcVKA5fEO2tZvKQm9g8Yj", Types: []string{"card", "link"}}, ref: "pi_1N4Zv32eZvKYlo2CPhVPkJlW"}
}

func (s *seller) Profile(context.Context, Caller, Quote) (*Profile, error) { return s.profile, nil }

func (s *seller) Take(_ context.Context, sale Sale) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sales = append(s.sales, sale)
	if s.err != nil {
		return "", s.err
	}
	return s.ref, nil
}

func (s *seller) taken() []Sale {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Sale(nil), s.sales...)
}

// The request the Stripe charge draft shows (section 6.2) is what the method makes for the
// quote and the profile it describes, and it decodes strictly into the request type.
func TestStripeRequestExample(t *testing.T) {
	var example string
	for _, b := range fences(t, "stripe-charge.md") {
		if b.lang == "json" && strings.Contains(b.text, `"networkId"`) {
			example = b.text
		}
	}
	if example == "" {
		t.Fatal("example not found")
	}
	var req request
	strict(t, []byte(example), &req)
	round, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, round, []byte(example))

	s := newSeller()
	p, err := Stripe{Seller: s, Store: &Memory{}}.Propose(context.Background(), Caller{}, Quote{
		Amount: "5000", Currency: "usd", Description: "Premium API access for 1 month", External: "order_12345",
	})
	if err != nil {
		t.Fatal(err)
	}
	made, err := json.Marshal(p.Request)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, made, []byte(example))
	// It is JCS on the wire, so it can be bound.
	wire, err := Encode(p.Request)
	if err != nil {
		t.Fatal(err)
	}
	decoded, _ := raw.DecodeString(wire)
	if canon, _ := Canonical(req); string(canon) != string(decoded) {
		t.Errorf("request on the wire is not JCS: %s", decoded)
	}
}

// The request examples of the charge intent draft (section 5.4) decode into Terms.
func TestChargeIntentExamples(t *testing.T) {
	n := 0
	for _, b := range fences(t, "intent-charge.md") {
		if b.lang != "json" || !strings.Contains(b.text, `"amount"`) {
			continue
		}
		var terms Terms
		strict(t, []byte(b.text), &terms)
		if !isAmount(terms.Amount) || terms.Currency == "" {
			t.Errorf("line %d: %+v", b.line, terms)
		}
		round, err := json.Marshal(terms)
		if err != nil {
			t.Fatal(err)
		}
		sameJSON(t, round, []byte(b.text))
		n++
	}
	if n != 3 {
		t.Fatalf("found %d request examples, want 3", n)
	}
}

func TestStripePayloadExample(t *testing.T) {
	var example string
	for _, b := range fences(t, "stripe-charge.md") {
		if b.lang == "json" && strings.Contains(b.text, `"externalId": "client_order_789"`) {
			example = b.text
		}
	}
	var pay payload
	strict(t, []byte(example), &pay)
	got, err := readPayload(json.RawMessage(example))
	if err != nil || got.SPT != "spt_1N4Zv32eZvKYlo2CPhVPkJlW" || got.ExternalID != "client_order_789" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestStripeVerify(t *testing.T) {
	s := Stripe{Seller: newSeller(), Store: &Memory{}}
	for name, p := range map[string]string{
		"no token":        `{}`,
		"empty token":     `{"spt":""}`,
		"not a token":     `{"spt":"pm_card_visa"}`,
		"token is number": `{"spt":1}`,
		"odd characters":  `{"spt":"spt_a b"}`,
		"too long":        `{"spt":"spt_` + strings.Repeat("a", 300) + `"}`,
		"external number": `{"spt":"spt_1","externalId":7}`,
		"not an object":   `[]`,
	} {
		err := s.Verify(context.Background(), Claim{Payload: json.RawMessage(p)})
		var pr *Problem
		if !errors.As(err, &pr) || pr.Code() != Payload || pr.Status != 402 {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := s.Verify(context.Background(), Claim{Payload: json.RawMessage(`{"spt":"spt_1N4Zv32eZvKYlo2CPhVPkJlW","future":true}`)}); err != nil {
		t.Errorf("unknown members must be ignored: %v", err)
	}
}

func TestStripePropose(t *testing.T) {
	ctx := context.Background()
	q := Quote{Amount: "5000", Currency: "usd"}
	ok := newSeller()
	for name, c := range map[string]struct {
		s    *seller
		q    Quote
		skip bool
	}{
		"usd":               {ok, q, false},
		"uppercase":         {ok, Quote{Amount: "1", Currency: "USD"}, true},
		"token address":     {ok, Quote{Amount: "1", Currency: "0x20c0000000000000000000000000000000000000"}, true},
		"not three letters": {ok, Quote{Amount: "1", Currency: "usdc"}, true},
		"not offered":       {&seller{}, q, true},
	} {
		_, err := Stripe{Seller: c.s, Store: &Memory{}}.Propose(ctx, Caller{}, c.q)
		if c.skip != errors.Is(err, ErrSkip) || (!c.skip && err != nil) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// A profile without the members the draft requires is the host's error, not a skip.
	_, err := Stripe{Seller: &seller{profile: &Profile{Network: "p"}}, Store: &Memory{}}.Propose(ctx, Caller{}, q)
	if err == nil || errors.Is(err, ErrSkip) {
		t.Errorf("profile without types: %v", err)
	}
	// Decimals ride along only when the seller states them.
	two := 2
	withDecimals := newSeller()
	withDecimals.profile.Decimals = &two
	p, _ := Stripe{Seller: withDecimals, Store: &Memory{}}.Propose(ctx, Caller{}, q)
	b, _ := json.Marshal(p.Request)
	if !strings.Contains(string(b), `"decimals":2`) {
		t.Errorf("%s", b)
	}
	p, _ = Stripe{Seller: ok, Store: &Memory{}}.Propose(ctx, Caller{}, q)
	b, _ = json.Marshal(p.Request)
	if strings.Contains(string(b), "decimals") {
		t.Errorf("%s", b)
	}
}

func stripeClaim(t *testing.T, id, spt, external string) Claim {
	t.Helper()
	req, _ := json.Marshal(request{
		Amount: "5000", Currency: "usd", Description: "d", ExternalID: "order_1",
		MethodDetails: &details{NetworkID: "profile_1", PaymentMethodTypes: []string{"card"}, Metadata: map[string]string{"plan": "pro"}},
	})
	pay := `{"spt":"` + spt + `"`
	if external != "" {
		pay += `,"externalId":"` + external + `"`
	}
	return Claim{
		Caller: Caller{Org: "acme", Subject: "agent"}, Challenge: Challenge{ID: id}, Request: req,
		Payload: json.RawMessage(pay + `}`), Until: epoch.Add(10 * time.Minute),
	}
}

// Section 9: the charge carries the request's amount and currency, the challenge id as
// metadata, the idempotency key challengeID_spt; the receipt names the PaymentIntent and
// echoes the payer's externalId (Table 5).
func TestStripeSettle(t *testing.T) {
	ctx := context.Background()
	s := newSeller()
	m := Stripe{Seller: s, Store: &Memory{Now: func() time.Time { return epoch }}}
	r, err := m.Settle(ctx, stripeClaim(t, "ch_1", "spt_1N4Zv32eZvKYlo2CPhVPkJlW", "client_order_789"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Method != "stripe" || r.Reference != "pi_1N4Zv32eZvKYlo2CPhVPkJlW" || r.Fields["externalId"] != "client_order_789" {
		t.Fatalf("%+v", r)
	}
	got := s.taken()
	if len(got) != 1 {
		t.Fatalf("%d sales", len(got))
	}
	want := Sale{
		Caller: Caller{Org: "acme", Subject: "agent"}, Token: "spt_1N4Zv32eZvKYlo2CPhVPkJlW", Amount: "5000", Currency: "usd", Description: "d",
		External: "order_1", Challenge: "ch_1", Key: "ch_1_spt_1N4Zv32eZvKYlo2CPhVPkJlW",
		Metadata: map[string]string{"plan": "pro", "challenge_id": "ch_1"},
	}
	if got[0].Key != want.Key || got[0].Token != want.Token || got[0].Amount != want.Amount || got[0].Currency != want.Currency ||
		got[0].External != want.External || got[0].Challenge != want.Challenge || got[0].Caller != want.Caller ||
		got[0].Metadata["plan"] != "pro" || got[0].Metadata["challenge_id"] != "ch_1" || len(got[0].Metadata) != 2 {
		t.Fatalf("sale %+v\nwant %+v", got[0], want)
	}
	// Without the payer's externalId the receipt has none.
	r, err = m.Settle(ctx, stripeClaim(t, "ch_2", "spt_second", ""))
	if err != nil || r.Fields != nil {
		t.Fatalf("%+v %v", r, err)
	}
}

// Section 8, step 4: a token is spent once, here and not at the Seller, even under
// another challenge.
func TestStripeTokenSpentOnce(t *testing.T) {
	s := newSeller()
	m := Stripe{Seller: s, Store: &Memory{Now: func() time.Time { return epoch }}}
	if _, err := m.Settle(context.Background(), stripeClaim(t, "ch_1", "spt_same", "")); err != nil {
		t.Fatal(err)
	}
	_, err := m.Settle(context.Background(), stripeClaim(t, "ch_2", "spt_same", ""))
	var p *Problem
	if !errors.As(err, &p) || p.Code() != Failed {
		t.Fatalf("%v", err)
	}
	if len(s.taken()) != 1 {
		t.Fatal("the seller was asked twice for one token")
	}
}

// A refusal of the Seller is passed on as it is; an error that is not one stays an
// unknown outcome.
func TestStripeSellerErrors(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		code Code
	}{
		"declined":      {Refuse(Failed, "card declined"), Failed},
		"action":        {Refuse(Action, "authenticate"), Action},
		"insufficient":  {Refuse(Insufficient, "over the token's limit"), Insufficient},
		"unknown":       {context.DeadlineExceeded, ""},
		"wrapped match": {errors.Join(errors.New("stripe"), Refuse(Failed, "x")), Failed},
	} {
		s := newSeller()
		s.err = c.err
		_, err := Stripe{Seller: s, Store: &Memory{Now: func() time.Time { return epoch }}}.Settle(context.Background(), stripeClaim(t, "ch_"+name, "spt_"+strings.ReplaceAll(name, " ", "_"), ""))
		var p *Problem
		isProblem := errors.As(err, &p)
		if (c.code == "") == isProblem || (isProblem && p.Code() != c.code) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestWallet(t *testing.T) {
	ctx := context.Background()
	s := newSeller()
	p, _ := Stripe{Seller: s, Store: &Memory{}}.Propose(ctx, Caller{}, Quote{Amount: "5000", Currency: "usd", Description: "AI", External: "order_9"})
	req, _ := Canonical(p.Request)
	ch := Challenge{ID: "a", Realm: "r", Method: "stripe", Intent: "charge", Expires: "2025-01-15T12:05:00Z"}

	var issued []Limits
	w := Wallet{
		Allow: func(_ Challenge, l Limits) error {
			if l.Currency != "usd" || l.Amount != "5000" {
				return errors.New("not what was expected")
			}
			return nil
		},
		Issue: func(_ context.Context, l Limits) (string, error) {
			issued = append(issued, l)
			return "spt_1N4Zv32eZvKYlo2CPhVPkJlW", nil
		},
	}
	ans, err := w.Pay(ctx, ch, req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(ans.Payload)
	sameJSON(t, b, []byte(`{"spt":"spt_1N4Zv32eZvKYlo2CPhVPkJlW"}`))
	want := Limits{Amount: "5000", Currency: "usd", Expires: time.Date(2025, 1, 15, 12, 5, 0, 0, time.UTC), Network: s.profile.Network,
		Types: s.profile.Types, Description: "AI", External: "order_9"}
	if len(issued) != 1 || issued[0].Amount != want.Amount || issued[0].Currency != want.Currency || !issued[0].Expires.Equal(want.Expires) ||
		issued[0].Network != want.Network || len(issued[0].Types) != 2 || issued[0].External != "order_9" || issued[0].Description != "AI" {
		t.Fatalf("limits %+v", issued)
	}
	if r := w.Accepts(); len(r) != 1 || r[0].String() != "stripe/charge" {
		t.Errorf("accepts %v", r)
	}

	// Nothing is issued unless the policy approves, and a wallet without one pays nothing.
	issued = nil
	picky := w
	picky.Allow = func(Challenge, Limits) error { return errors.New("too much") }
	if _, err := picky.Pay(ctx, ch, req); err == nil || len(issued) != 0 {
		t.Errorf("refused payment: %v, %d issued", err, len(issued))
	}
	none := w
	none.Allow = nil
	if _, err := none.Pay(ctx, ch, req); err == nil || len(issued) != 0 {
		t.Errorf("wallet with no policy: %v", err)
	}
	bad := w
	bad.Issue = func(context.Context, Limits) (string, error) { return "pm_card", nil }
	if _, err := bad.Pay(ctx, ch, req); err == nil {
		t.Error("a value that is not a token was sent")
	}
	other := ch
	other.Method = "tempo"
	if _, err := w.Pay(ctx, other, req); err == nil {
		t.Error("answered another method")
	}
	for name, r := range map[string]string{
		"no details":     `{"amount":"5000","currency":"usd"}`,
		"no types":       `{"amount":"5000","currency":"usd","methodDetails":{"networkId":"p","paymentMethodTypes":[]}}`,
		"bad amount":     `{"amount":"50.00","currency":"usd","methodDetails":{"networkId":"p","paymentMethodTypes":["card"]}}`,
		"bad currency":   `{"amount":"5000","currency":"USD","methodDetails":{"networkId":"p","paymentMethodTypes":["card"]}}`,
		"overflowing":    `{"amount":"99999999999999999999","currency":"usd","methodDetails":{"networkId":"p","paymentMethodTypes":["card"]}}`,
		"not an object":  `[]`,
		"amount numeric": `{"amount":5000,"currency":"usd","methodDetails":{"networkId":"p","paymentMethodTypes":["card"]}}`,
	} {
		if _, err := w.Pay(ctx, ch, json.RawMessage(r)); err == nil {
			t.Errorf("%s: paid", name)
		}
	}
	if len(issued) != 0 {
		t.Errorf("%d tokens issued by calls that must issue none", len(issued))
	}
}
