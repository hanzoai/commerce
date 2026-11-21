// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/models/organization"
)

// checkout stands in for api/billing's checkout and records how it was called.
type checkout struct {
	mu        sync.Mutex
	org       *organization.Organization
	items     [][]billing.ItemIn
	subjects  []string
	completed []completion
	spent     []spending
	spendErr  error
	fault     *billing.PaymentFault
	out       billing.CompleteOut
}

// spending is what Commerce drew from the credit it had just granted.
type spending struct {
	subject string
	cents   int64
	ref     string
}

type completion struct {
	id, key string
	pay     billing.PaymentIn
}

func newCheckout() *checkout {
	return &checkout{
		org: new(organization.Organization),
		out: billing.CompleteOut{Status: 200, View: &billing.CheckoutView{ID: "chk_1", Status: "complete", Order: &billing.OrderView{ID: "chk_1", PaymentID: "txn_1"}}},
	}
}

func (c *checkout) commerce() *Commerce {
	return &Commerce{
		Store: &memory{},
		Account: func(context.Context, Caller) (*Profile, error) {
			return &Profile{Network: "profile_1MqDcVKA5fEO2tZvKQm9g8Yj", Types: []string{"card", "link"}}, nil
		},
		Orgs: func(_ context.Context, name string) (*organization.Organization, error) {
			if name != "acme" {
				return nil, errors.New("no such organization")
			}
			return c.org, nil
		},
		Create: func(_ context.Context, org *organization.Organization, subject, _ string, items []billing.ItemIn) (*billing.CheckoutView, *billing.PaymentFault) {
			c.mu.Lock()
			defer c.mu.Unlock()
			if org != c.org {
				return nil, &billing.PaymentFault{Status: 500, Message: "wrong org"}
			}
			c.items, c.subjects = append(c.items, items), append(c.subjects, subject)
			if c.fault != nil {
				return nil, c.fault
			}
			return &billing.CheckoutView{ID: "chk_1", Status: "open"}, nil
		},
		Complete: func(_ context.Context, _ *organization.Organization, _, _, id, key string, pay billing.PaymentIn) billing.CompleteOut {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.completed = append(c.completed, completion{id, key, pay})
			return c.out
		},
		Spend: func(_ context.Context, org *organization.Organization, subject string, cents int64, ref string) error {
			c.mu.Lock()
			defer c.mu.Unlock()
			if org != c.org {
				return errors.New("wrong org")
			}
			c.spent = append(c.spent, spending{subject, cents, ref})
			return c.spendErr
		},
	}
}

func TestCommerceProfile(t *testing.T) {
	c := newCheckout().commerce()
	ctx := context.Background()
	who := Caller{Org: "acme", Subject: "agent"}
	if p, err := c.Profile(ctx, who, Quote{Amount: "5000", Currency: "usd"}); err != nil || p == nil || p.Network != "profile_1MqDcVKA5fEO2tZvKQm9g8Yj" {
		t.Fatalf("%+v %v", p, err)
	}
	for name, c := range map[string]struct {
		who Caller
		q   Quote
	}{
		"anonymous":                  {Caller{Org: "acme"}, Quote{Amount: "1", Currency: "usd"}},
		"no org":                     {Caller{Subject: "agent"}, Quote{Amount: "1", Currency: "usd"}},
		"euros":                      {who, Quote{Amount: "1", Currency: "eur"}},
		"no account":                 {who, Quote{Amount: "1", Currency: "usd"}},
		"no way to spend the credit": {who, Quote{Amount: "1", Currency: "usd"}},
	} {
		s := newCheckout().commerce()
		if name == "no account" {
			s.Account = nil
		}
		if name == "no way to spend the credit" {
			s.Spend = nil // and no ledger is injected
		}
		if _, err := s.Profile(ctx, c.who, c.q); !errors.Is(err, ErrSkip) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// The checkout sells USD credit for the quoted cents to the caller's subject, and is
// completed on the token under the challenge id.
func TestCommerceTake(t *testing.T) {
	k := newCheckout()
	ref, err := k.commerce().Take(context.Background(), Sale{
		Caller: Caller{Org: "acme", Subject: "agent"}, Token: "spt_1", Amount: "5000", Currency: "usd", Challenge: "ch_1", Key: "ch_1_spt_1",
	})
	if err != nil || ref != "txn_1" {
		t.Fatalf("%q %v", ref, err)
	}
	if len(k.items) != 1 || len(k.items[0]) != 1 || k.items[0][0] != (billing.ItemIn{Kind: "credit", UsdCents: 5000}) || k.subjects[0] != "agent" {
		t.Fatalf("created %+v for %v", k.items, k.subjects)
	}
	if len(k.completed) != 1 || k.completed[0] != (completion{"chk_1", "ch_1", billing.PaymentIn{Type: "shared_payment_token", Token: "spt_1"}}) {
		t.Fatalf("completed %+v", k.completed)
	}
	// The resource's price is spent from the credit the checkout granted, under the challenge.
	if len(k.spent) != 1 || k.spent[0] != (spending{"agent", 5000, "ch_1"}) {
		t.Fatalf("spent %+v", k.spent)
	}
	// An order with no ledger transaction is referenced by its own id.
	k.out.View.Order.PaymentID = ""
	if ref, err := k.commerce().Take(context.Background(), Sale{Caller: Caller{Org: "acme", Subject: "a"}, Token: "spt_2", Amount: "5000", Currency: "usd", Challenge: "ch_2"}); err != nil || ref != "chk_1" {
		t.Fatalf("%q %v", ref, err)
	}
}

func TestCommerceTakeRefusals(t *testing.T) {
	sale := Sale{Caller: Caller{Org: "acme", Subject: "agent"}, Token: "spt_1", Amount: "5000", Currency: "usd", Challenge: "ch_1"}
	declined := func(status int, code string) billing.CompleteOut {
		return billing.CompleteOut{Status: status, View: &billing.CheckoutView{ID: "chk_1", Status: "failed", Error: &billing.AgentError{Code: code, Message: code + " message"}}}
	}
	for name, c := range map[string]struct {
		out  billing.CompleteOut
		code Code // "" for an outcome that is not a refusal
	}{
		"bank step":         {declined(402, "authentication_required"), Action},
		"token over limit":  {declined(402, "token_over_limit"), Insufficient},
		"token expired":     {declined(402, "token_expired"), Expired},
		"token revoked":     {declined(402, "token_revoked"), Failed},
		"wrong merchant":    {declined(402, "token_wrong_merchant"), Failed},
		"declined":          {declined(402, "declined"), Failed},
		"payments down":     {declined(402, "payments_unavailable"), Internal},
		"in progress":       {declined(409, "in_progress"), ""},
		"unavailable":       {declined(503, "unavailable"), ""},
		"pending":           {billing.CompleteOut{Status: 202, View: &billing.CheckoutView{ID: "chk_1", Status: "pending"}}, ""},
		"complete no order": {billing.CompleteOut{Status: 200, View: &billing.CheckoutView{ID: "chk_1"}}, ""},
		"no view":           {billing.CompleteOut{Status: 500}, ""},
	} {
		k := newCheckout()
		k.out = c.out
		_, err := k.commerce().Take(context.Background(), sale)
		var p *Problem
		isProblem := errors.As(err, &p)
		if err == nil || (c.code == "") == isProblem || (isProblem && codeOf(p) != c.code) {
			t.Errorf("%s: %v", name, err)
		}
		// Nothing is spent for a payment that did not complete.
		if len(k.spent) != 0 {
			t.Errorf("%s: spent %+v", name, k.spent)
		}
	}
	// Refused before the checkout: the currency, the amount, the organization; and a
	// checkout that cannot be made is the host's to look at.
	k := newCheckout()
	cm := k.commerce()
	ctx := context.Background()
	bad := sale
	bad.Currency = "eur"
	if _, err := cm.Take(ctx, bad); !isCode(err, Unsupported) {
		t.Errorf("euros: %v", err)
	}
	bad = sale
	bad.Amount = "12.50"
	if _, err := cm.Take(ctx, bad); !isCode(err, Bad) {
		t.Errorf("decimal amount: %v", err)
	}
	bad = sale
	bad.Caller.Org = "nobody"
	if _, err := cm.Take(ctx, bad); err == nil || errors.As(err, new(*Problem)) {
		t.Errorf("unknown org: %v", err)
	}
	k.fault = &billing.PaymentFault{Status: 400, Message: "usdCents must be between 500 and 500000"}
	if _, err := cm.Take(ctx, sale); err == nil || errors.As(err, new(*Problem)) {
		t.Errorf("checkout refused: %v", err)
	}
	if len(k.completed) != 0 {
		t.Error("completed a checkout that was never made")
	}
}

func isCode(err error, c Code) bool {
	var p *Problem
	return errors.As(err, &p) && (c == "" || codeOf(p) == c)
}

// The whole path: a request is challenged at the host's price, the credential is
// verified and settled by Stripe on Commerce, and the checkout is for the host's amount,
// whatever the client sends.
func TestCommerceThroughTheGate(t *testing.T) {
	k := newCheckout()
	r := newRig(t, func(c *Config) {
		c.Methods = []Method{Stripe{Seller: k.commerce(), Store: &memory{}}}
		c.Caller = func(req *http.Request) (Caller, error) {
			if req.Header.Get("X-Subject") == "" {
				return Caller{Org: "acme"}, nil
			}
			return Caller{Org: "acme", Subject: req.Header.Get("X-Subject")}, nil
		}
		c.Price = func(*http.Request) (*Quote, error) {
			return &Quote{Amount: "5000", Currency: "usd", Description: "Credit"}, nil
		}
	})
	// A caller with no subject is offered no way to pay.
	if resp := r.do("GET", "/resource", nil, ""); resp.StatusCode != 403 {
		t.Fatalf("anonymous: %d", resp.StatusCode)
	}
	who := http.Header{"X-Subject": {"agent-1"}}
	ch := r.challenge("GET", "/resource", who, "")[0]
	if ch.Method != "stripe" {
		t.Fatalf("method %s", ch.Method)
	}
	hdr := http.Header{"X-Subject": {"agent-1"}, "Authorization": {credential(ch, `{"spt":"spt_1N4Zv32eZvKYlo2CPhVPkJlW"}`).String()}}
	resp := r.do("GET", "/resource", hdr, "")
	if resp.StatusCode != 200 {
		t.Fatalf("paid: %d", resp.StatusCode)
	}
	rcpt, err := ParseReceipt(resp.Header.Get("Payment-Receipt"))
	if err != nil || rcpt.Method != "stripe" || rcpt.Reference != "txn_1" {
		t.Fatalf("receipt %+v %v", rcpt, err)
	}
	if len(k.items) != 1 || k.items[0][0].UsdCents != 5000 || k.subjects[0] != "agent-1" || k.completed[0].key != ch.ID || k.completed[0].pay.Token != "spt_1N4Zv32eZvKYlo2CPhVPkJlW" {
		t.Fatalf("checkout %+v %v %+v", k.items, k.subjects, k.completed)
	}
	if len(k.spent) != 1 || k.spent[0] != (spending{"agent-1", 5000, ch.ID}) {
		t.Fatalf("the credit was not spent on the resource: %+v", k.spent)
	}
}

// A charge whose credit cannot be spent serves nothing: the host is told, the credit stays
// with the buyer.
func TestCommerceSpendFailure(t *testing.T) {
	k := newCheckout()
	k.spendErr = errors.New("ledger down")
	_, err := k.commerce().Take(context.Background(), Sale{Caller: Caller{Org: "acme", Subject: "agent"}, Token: "spt_1", Amount: "5000", Currency: "usd", Challenge: "ch_1"})
	if err == nil || errors.As(err, new(*Problem)) {
		t.Fatalf("%v", err)
	}
	if !strings.Contains(err.Error(), "not spent") || !strings.Contains(err.Error(), "ch_1") {
		t.Errorf("the host is not told what happened: %v", err)
	}
}

// ledger is the credit ledger a host injects; only the debit is asked of it here.
type ledger struct {
	creditledger.CreditLedger
	debits []creditledger.DebitInput
	err    error
}

func (l *ledger) Debit(_ context.Context, in creditledger.DebitInput) (string, int64, error) {
	l.debits = append(l.debits, in)
	return "dbt_1", 0, l.err
}

// With no Spend of the host's, the credit is drawn through the ledger api/billing credited:
// the org's name as the ledger keys it, the subject, USD, once per challenge.
func TestCommerceSpendsThroughTheLedger(t *testing.T) {
	l := &ledger{}
	creditledger.Set(l)
	t.Cleanup(func() { creditledger.Set(nil) })
	k := newCheckout()
	k.org.Name = " Acme "
	c := k.commerce()
	c.Spend = nil
	if _, err := c.Profile(context.Background(), Caller{Org: "acme", Subject: "agent"}, Quote{Amount: "5000", Currency: "usd"}); err != nil {
		t.Fatalf("a ledger is injected, so it is offered: %v", err)
	}
	if _, err := c.Take(context.Background(), Sale{Caller: Caller{Org: "acme", Subject: "agent"}, Token: "spt_1", Amount: "5000", Currency: "usd", Challenge: "ch_1"}); err != nil {
		t.Fatal(err)
	}
	want := creditledger.DebitInput{Org: "acme", Subject: "agent", Currency: "usd", Reason: "mpp ch_1", Ref: "ch_1", AmountCents: 5000, Test: k.org.TestMode()}
	if len(l.debits) != 1 || l.debits[0] != want {
		t.Fatalf("debits %+v, want %+v", l.debits, want)
	}
	// A ledger that cannot cover it serves nothing.
	l.err = creditledger.ErrShort
	if _, err := c.Take(context.Background(), Sale{Caller: Caller{Org: "acme", Subject: "agent"}, Token: "spt_2", Amount: "5000", Currency: "usd", Challenge: "ch_2"}); err == nil {
		t.Fatal("served without the credit being spent")
	}
}

// The checkout is the challenge's: an attempt after the first, on this process or another,
// completes the checkout the first opened, under the same key, and opens none (api/billing
// binds the charge to checkout:<id>:<key>, so the card is charged once).
func TestCommerceCheckoutIsTheChallenges(t *testing.T) {
	k := newCheckout()
	c := k.commerce()
	sale := Sale{Caller: Caller{Org: "acme", Subject: "agent"}, Token: "spt_1", Amount: "5000", Currency: "usd", Challenge: "ch_1", Until: epoch.Add(10 * time.Minute)}
	first, err := c.Take(context.Background(), sale)
	if err != nil {
		t.Fatal(err)
	}
	again, err := c.Take(context.Background(), sale)
	if err != nil || again != first {
		t.Fatalf("%q %v", again, err)
	}
	if len(k.items) != 1 {
		t.Fatalf("%d checkouts were opened for one challenge", len(k.items))
	}
	want := completion{"chk_1", "ch_1", billing.PaymentIn{Type: "shared_payment_token", Token: "spt_1"}}
	if len(k.completed) != 2 || k.completed[0] != want || k.completed[1] != want {
		t.Fatalf("completed %+v", k.completed)
	}
	// Another challenge opens its own.
	other := sale
	other.Challenge = "ch_2"
	if _, err := c.Take(context.Background(), other); err != nil || len(k.items) != 2 {
		t.Fatalf("%v, %d checkouts", err, len(k.items))
	}
	// The record of an opened checkout that could not be kept is the host's to see.
	c.Store = brokenStore{&memory{}}
	if _, err := c.Take(context.Background(), Sale{Caller: sale.Caller, Token: "spt_3", Amount: "5000", Currency: "usd", Challenge: "ch_3"}); err == nil {
		t.Error("a checkout opened and not recorded was not reported")
	}
	if _, err := (&Commerce{Account: c.Account}).Profile(context.Background(), sale.Caller, Quote{Amount: "1", Currency: "usd"}); err == nil || errors.Is(err, ErrSkip) {
		t.Errorf("a Commerce with no Store: %v", err)
	}
}

type brokenStore struct{ *memory }

func (brokenStore) Load(context.Context, string) ([]byte, bool, error) { return nil, false, nil }
func (brokenStore) Save(context.Context, string, []byte, time.Time) error {
	return errors.New("store down")
}
