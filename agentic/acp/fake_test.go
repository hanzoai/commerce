// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fake is a Backend that holds checkouts in memory, for the seller named by the caller.
type fake struct {
	mu       sync.Mutex
	items    map[string]Product
	price    map[string]int64
	sessions map[string]*Checkout
	owner    map[string]Caller
	paid     map[string]string // session id -> key that paid it
	n        int
	opens    int
	pays     int
	gate     chan struct{} // when set, Open waits for it to close
	entered  chan struct{} // signalled when Open is waiting at the gate
	openErr  error         // returned by the next Open, once
	refuse   *Refusal      // the next Pay is refused with this
}

func newFake() *fake {
	return &fake{
		items:    map[string]Product{"item_123": {ID: "item_123", Name: "Vintage Denim Jacket"}, "item_456": {ID: "item_456", Name: "Canvas Tote"}},
		price:    map[string]int64{"item_123": 300, "item_456": 500},
		sessions: map[string]*Checkout{},
		owner:    map[string]Caller{},
		paid:     map[string]string{},
	}
}

func (f *fake) Currency() string { return "usd" }

func (f *fake) get(who Caller, id string) (*Checkout, error) {
	c, ok := f.sessions[id]
	if !ok || f.owner[id] != who {
		return nil, NotFound("checkout not found")
	}
	return c, nil
}

func (f *fake) Open(_ context.Context, who Caller, item string) (*Checkout, error) {
	f.mu.Lock()
	gate := f.gate
	f.mu.Unlock()
	if gate != nil {
		f.entered <- struct{}{}
		<-gate
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opens++
	if err := f.openErr; err != nil {
		f.openErr = nil
		return nil, err
	}
	p, ok := f.items[item]
	if !ok {
		return nil, ErrNoItem
	}
	f.n++
	id := fmt.Sprintf("checkout_session_%d", f.n)
	c := &Checkout{ID: id, State: Open, Item: p, Total: f.price[item]}
	f.sessions[id], f.owner[id] = c, who
	cp := *c
	return &cp, nil
}

func (f *fake) Change(_ context.Context, who Caller, id, item string) (*Checkout, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, err := f.get(who, id)
	if err != nil {
		return nil, err
	}
	p, ok := f.items[item]
	if !ok {
		return nil, ErrNoItem
	}
	if c.State != Open && c.State != Failed {
		return nil, Conflict("this checkout is " + string(c.State) + " and can no longer be changed")
	}
	c.Item, c.Total, c.State, c.Refusal = p, f.price[item], Open, nil
	cp := *c
	return &cp, nil
}

func (f *fake) Read(_ context.Context, who Caller, id string) (*Checkout, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, err := f.get(who, id)
	if err != nil {
		return nil, err
	}
	cp := *c
	return &cp, nil
}

func (f *fake) Pay(_ context.Context, who Caller, id, key string, pay Charge) (*Checkout, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, err := f.get(who, id)
	if err != nil {
		return nil, err
	}
	if pay.Instrument != "card" || pay.Credential != "spt" || !strings.HasPrefix(pay.Token, "spt_") {
		return nil, Unsupported("$.payment_data.instrument", "only card shared payment tokens")
	}
	switch c.State {
	case Canceled:
		return nil, Conflict("the checkout session is canceled")
	case Paid:
		if f.paid[id] != key {
			return nil, Conflict("this checkout was completed under another idempotency key")
		}
		cp := *c
		return &cp, nil
	}
	f.pays++
	if r := f.refuse; r != nil {
		f.refuse = nil
		c.State, c.Refusal = Failed, r
		cp := *c
		return &cp, nil
	}
	c.State, c.Refusal, c.Receipt = Paid, nil, &Receipt{ID: "ord_" + id}
	f.paid[id] = key
	cp := *c
	return &cp, nil
}

func (f *fake) Cancel(_ context.Context, who Caller, id string) (*Checkout, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, err := f.get(who, id)
	if err != nil {
		return nil, err
	}
	if c.State != Open && c.State != Failed {
		return nil, NotCancelable("The checkout session is " + string(c.State) + " and cannot be canceled.")
	}
	c.State = Canceled
	cp := *c
	return &cp, nil
}

// events records what the webhook was asked to send.
type events struct {
	mu  sync.Mutex
	got []Event
	err error
}

func (e *events) Send(_ context.Context, ev Event) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.got = append(e.got, ev)
	return e.err
}

// reports records what the service reported instead of answering.
type reports struct {
	mu   sync.Mutex
	errs []error
}

func (r *reports) add(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errs = append(r.errs, err)
}

// rig is a Service over a fake, with the caller "agent_key" acting for buyer_1 of acme.
type rig struct {
	t       *testing.T
	svc     *Service
	h       http.Handler
	be      *fake
	ev      *events
	rep     *reports
	handler PaymentHandler
	now     time.Time
	cfg     Config
}

var (
	buyer1 = Caller{Org: "acme", Subject: "buyer_1", Host: "shop.example"}
	buyer2 = Caller{Org: "acme", Subject: "buyer_2", Host: "shop.example"}
)

func newRig(t *testing.T, mut ...func(*Config)) *rig {
	t.Helper()
	h, err := TokenizedCard("card_tokenized", "acct_1234567890", "stripe", false, []string{"visa", "mastercard"}, []string{"credit", "debit"})
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{t: t, be: newFake(), ev: &events{}, rep: &reports{}, handler: h, now: time.Date(2026, 4, 17, 10, 30, 0, 0, time.UTC)}
	r.cfg = Config{
		Backend: r.be,
		Identify: func(req *http.Request) (Caller, error) {
			switch strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ") {
			case "agent_key":
				return buyer1, nil
			case "other_key":
				return buyer2, nil
			}
			return Caller{}, errors.New("unknown token")
		},
		Handlers: func(context.Context, Caller) ([]PaymentHandler, error) { return []PaymentHandler{r.handler}, nil },
		Order:    func(id string) string { return "https://www.testshop.com/orders/" + id },
		Links:    []Link{{Type: "terms_of_use", URL: "https://www.testshop.com/legal/terms-of-use"}},
		Events:   r.ev,
		Report:   r.rep.add,
		Now:      func() time.Time { return r.now },
	}
	for _, m := range mut {
		m(&r.cfg)
	}
	r.svc, err = New(r.cfg)
	if err != nil {
		t.Fatal(err)
	}
	r.h = r.svc.API()
	return r
}

// call is one request.
type call struct {
	method, path, body string
	hdr                map[string]string
}

// res is a response.
type res struct {
	*httptest.ResponseRecorder
}

func (r res) json() map[string]any {
	var m map[string]any
	if err := json.Unmarshal(r.Body.Bytes(), &m); err != nil {
		panic(fmt.Sprintf("not JSON: %v: %s", err, r.Body.String()))
	}
	return m
}

func (r res) get(path ...string) any {
	var v any = r.json()
	for _, p := range path {
		switch x := v.(type) {
		case map[string]any:
			v = x[p]
		default:
			return nil
		}
	}
	return v
}

// send runs a request with the standard headers of a well-formed agent, which hdr overrides;
// an empty value removes a header.
func (r *rig) send(c call) res {
	r.t.Helper()
	req := httptest.NewRequest(c.method, c.path, bytes.NewBufferString(c.body))
	req.Header.Set("Authorization", "Bearer agent_key")
	req.Header.Set("API-Version", Version)
	if c.body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.method == http.MethodPost {
		sum := sha256.Sum256([]byte(c.method + c.path + c.body))
		req.Header.Set("Idempotency-Key", "key-"+hex.EncodeToString(sum[:8]))
	}
	for k, v := range c.hdr {
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	w := httptest.NewRecorder()
	r.h.ServeHTTP(w, req)
	return res{w}
}

func (r *rig) post(path, body string, hdr ...map[string]string) res {
	r.t.Helper()
	c := call{method: http.MethodPost, path: path, body: body}
	if len(hdr) > 0 {
		c.hdr = hdr[0]
	}
	return r.send(c)
}

func (r *rig) get(path string, hdr ...map[string]string) res {
	r.t.Helper()
	c := call{method: http.MethodGet, path: path}
	if len(hdr) > 0 {
		c.hdr = hdr[0]
	}
	return r.send(c)
}

// create opens a session for item_123 and returns its id.
func (r *rig) create() string {
	r.t.Helper()
	w := r.post("/checkout_sessions", createBody, map[string]string{"Idempotency-Key": fmt.Sprint("create-", r.be.n)})
	if w.Code != http.StatusCreated {
		r.t.Fatalf("create = %d %s", w.Code, w.Body)
	}
	return w.get("id").(string)
}

const createBody = `{"currency":"usd","line_items":[{"id":"item_123"}],"capabilities":{"interventions":{"supported":["3ds","address_verification"]}}}`

const completeBody = `{"payment_data":{"handler_id":"card_tokenized","instrument":{"type":"card","credential":{"type":"spt","token":"spt_123"}}}}`
