// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// maxChallenge is the size a server keeps a challenge under (section 9.4).
const maxChallenge = 8192

// Config is what a Gate takes from its host.
type Config struct {
	// Realm is the protection space named in challenges (section 5.1.1).
	Realm string
	// Secret keys the HMAC-SHA256 challenge binding (section 5.1.2.1.1). At least
	// sha256.Size bytes (RFC 2104 section 3).
	Secret []byte
	// Previous are retired secrets, still verified until their challenges expire
	// (section 11.2.2).
	Previous [][]byte
	// Store holds spent credentials. It is shared by every process serving the resource.
	Store Store
	// Methods are the offered payment methods, in the server's order of preference.
	Methods []Method
	// Price is what a request pays, or nil when it pays nothing. It may return
	// ErrUnauthorized or ErrDenied.
	Price func(*http.Request) (*Quote, error)
	// Caller is who a request is for; nil is anonymous. It may return ErrUnauthorized
	// or ErrDenied.
	Caller func(*http.Request) (Caller, error)
	// Secure says a request arrived over TLS (section 11.2); nil is r.TLS != nil.
	Secure func(*http.Request) bool
	// Lifetime is how long a challenge is valid, from which expires is set (section 5.1.2).
	Lifetime time.Duration
	// Digest, when positive, binds request bodies of up to this many bytes to the
	// challenge (section 5.1.3). A larger body is refused with 413.
	Digest int64
	// Report is told of outcomes that cannot be answered to the client.
	Report func(error)
	// Now is the clock; nil is time.Now.
	Now func() time.Time
}

// Gate is net/http middleware that puts a resource behind the Payment scheme.
type Gate struct {
	c Config
}

// New checks a configuration and returns its Gate.
func New(c Config) (*Gate, error) {
	switch {
	case c.Realm == "":
		return nil, errors.New("mpp: realm is required")
	case len(c.Secret) < sha256.Size:
		return nil, fmt.Errorf("mpp: secret must be at least %d bytes", sha256.Size)
	case c.Store == nil:
		return nil, errors.New("mpp: store is required")
	case len(c.Methods) == 0:
		return nil, errors.New("mpp: at least one method is required")
	case c.Price == nil:
		return nil, errors.New("mpp: price is required")
	case c.Report == nil:
		return nil, errors.New("mpp: report is required")
	case c.Lifetime <= 0:
		return nil, errors.New("mpp: lifetime is required")
	case c.Digest < 0:
		return nil, errors.New("mpp: digest cannot be negative")
	}
	for _, s := range c.Previous {
		if len(s) < sha256.Size {
			return nil, fmt.Errorf("mpp: a previous secret is shorter than %d bytes", sha256.Size)
		}
	}
	seen := map[string]bool{}
	for _, m := range c.Methods {
		n := m.Name()
		if !methodShape.MatchString(n) || seen[n] {
			return nil, fmt.Errorf("mpp: method %q is not a unique lowercase identifier", n)
		}
		seen[n] = true
	}
	if c.Secure == nil {
		c.Secure = func(r *http.Request) bool { return r.TLS != nil }
	}
	if c.Caller == nil {
		c.Caller = func(*http.Request) (Caller, error) { return Caller{}, nil }
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return &Gate{c}, nil
}

// Wrap serves next only for a request that has paid. Caller and Price run first
// (section 4.4.3); then a request with no credential gets a 402 with a challenge per
// method, and one with a credential has it verified and settled once before next runs.
func (g *Gate) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { g.serve(w, r, next) })
}

func (g *Gate) serve(w http.ResponseWriter, r *http.Request, next http.Handler) {
	caller, err := g.c.Caller(r)
	if err != nil {
		g.refuse(w, err)
		return
	}
	quote, err := g.c.Price(r)
	if err != nil {
		g.refuse(w, err)
		return
	}
	if quote == nil {
		next.ServeHTTP(w, r)
		return
	}
	if err := quote.Valid(); err != nil {
		g.fail(w, err)
		return
	}
	if !g.c.Secure(r) {
		g.write(w, Refuse(Bad, "Payment requires TLS."))
		return
	}
	body, p := g.body(r)
	if p != nil {
		g.write(w, p)
		return
	}
	switch creds := credentials(r); len(creds) {
	case 0:
		g.challenge(w, r, caller, *quote, body, nil)
	case 1:
		g.redeem(w, r, next, caller, *quote, creds[0], body)
	default:
		g.write(w, Refuse(Bad, "More than one Payment credential."))
	}
}

// held is a Payment credential found on a request and the field it arrived in, or why it
// could not be read.
type held struct {
	cred  *Credential
	field string
	err   error
}

// isPayment says a field value is of the Payment scheme.
func isPayment(v string) bool {
	scheme, _, _ := strings.Cut(strings.TrimSpace(v), " ")
	return strings.EqualFold(scheme, Scheme)
}

// credentials lists the Payment credentials in Authorization and Payment-Authorization
// (section 5.2).
func credentials(r *http.Request) []held {
	var out []held
	for _, field := range []string{authorization, alternate} {
		for _, v := range r.Header.Values(field) {
			if isPayment(v) {
				c, err := ParseCredential(v)
				out = append(out, held{c, field, err})
			}
		}
	}
	return out
}

// otherAuth says a request carries authentication of another scheme in Authorization, so
// the credential must travel in Payment-Authorization (section 4.4.3).
func otherAuth(r *http.Request) bool {
	for _, v := range r.Header.Values(authorization) {
		if !isPayment(v) {
			return true
		}
	}
	return false
}

// body buffers a request body up to Config.Digest and puts it back for the resource.
func (g *Gate) body(r *http.Request) ([]byte, *Problem) {
	if g.c.Digest == 0 || r.Body == nil || r.Body == http.NoBody {
		return nil, nil
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, g.c.Digest+1))
	_ = r.Body.Close()
	if err != nil {
		return nil, Refuse(Bad, "The request body could not be read.")
	}
	if int64(len(b)) > g.c.Digest {
		p := Refuse(Bad, "The request body is too large to bind to a payment.")
		p.Status = http.StatusRequestEntityTooLarge
		return nil, p
	}
	r.Body = io.NopCloser(bytes.NewReader(b))
	return b, nil
}

// proposed is a method and the request it makes for a quote.
type proposed struct {
	method   Method
	proposal *Proposal
}

// proposals asks each method for its request; a method returning ErrSkip offers none.
func (g *Gate) proposals(ctx context.Context, caller Caller, q Quote) ([]proposed, error) {
	var out []proposed
	for _, m := range g.c.Methods {
		p, err := m.Propose(ctx, caller, q)
		if errors.Is(err, ErrSkip) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", m.Name(), err)
		}
		out = append(out, proposed{m, p})
	}
	return out, nil
}

// issue makes a challenge for each method that offers one. Its opaque holds a random nonce,
// which makes each id unique, and the route and a keyed tag of the caller, which bind the
// challenge to the request and caller it was issued to.
func (g *Gate) issue(r *http.Request, caller Caller, q Quote, body []byte) ([]Challenge, error) {
	props, err := g.proposals(r.Context(), caller, q)
	if err != nil {
		return nil, err
	}
	expires := g.c.Now().UTC().Add(g.c.Lifetime).Format(time.RFC3339)
	var out []Challenge
	for _, p := range props {
		name := p.method.Name()
		req, err := Encode(p.proposal.Request)
		if err != nil {
			return nil, fmt.Errorf("%s: request: %w", name, err)
		}
		if _, t, err := termsOf(req); err != nil || t.Amount != q.Amount || t.Currency != q.Currency || t.ExternalID != q.External {
			return nil, fmt.Errorf("%s: request does not carry the quoted amount, currency and external id", name)
		}
		nonce := make([]byte, sha256.Size)
		if _, err := rand.Read(nonce); err != nil {
			return nil, err
		}
		c := Challenge{Realm: g.c.Realm, Method: name, Intent: Charge, Request: req, Expires: expires, Description: q.Description}
		opaque := map[string]string{"nonce": base64.RawURLEncoding.EncodeToString(nonce), "route": route(r), "caller": tag(g.c.Secret, caller)}
		if c.Opaque, err = Encode(opaque); err != nil {
			return nil, fmt.Errorf("%s: opaque: %w", name, err)
		}
		if otherAuth(r) {
			c.Header = alternate
		}
		if len(body) > 0 {
			c.Digest = digestOf(body)
		}
		c.ID = Bind(g.c.Secret, c)
		if err := c.Valid(); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if len(c.String()) > maxChallenge {
			return nil, fmt.Errorf("%s: challenge is larger than %d bytes", name, maxChallenge)
		}
		out = append(out, c)
	}
	return out, nil
}

// route is what a request asks for: its method and path.
func route(r *http.Request) string { return r.Method + " " + r.URL.Path }

// tag is a keyed digest of the caller: a challenge says which caller it is for without
// saying who that is.
func tag(secret []byte, who Caller) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(who.Org + "\x00" + who.Subject))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// preferred orders challenges by the request's Accept-Payment header, ignoring a header
// that is malformed or matches none (section 7.4).
func preferred(r *http.Request, cs []Challenge) []Challenge {
	ranges, err := parseAccept(r.Header.Values(accepting))
	if err != nil || len(ranges) == 0 {
		return cs
	}
	offers := make([][2]string, len(cs))
	for i, c := range cs {
		offers[i] = [2]string{c.Method, c.Intent}
	}
	order, err := rank(offers, ranges)
	if err != nil {
		return cs
	}
	out := make([]Challenge, len(order))
	for i, n := range order {
		out[i] = cs[n]
	}
	return out
}

// challenge answers 402 with a challenge per method and the problem given,
// payment-required when none (section 4.2).
func (g *Gate) challenge(w http.ResponseWriter, r *http.Request, caller Caller, q Quote, body []byte, p *Problem) {
	cs, err := g.issue(r, caller, q, body)
	if err != nil {
		g.fail(w, err)
		return
	}
	if len(cs) == 0 {
		g.write(w, plain(http.StatusForbidden, "No payment method is available for this request."))
		return
	}
	cs = preferred(r, cs)
	if p == nil {
		p = Refuse(Required, "Payment required for access.")
		if len(cs) == 1 {
			p.Challenge = cs[0].ID
		}
	}
	for _, c := range cs {
		w.Header().Add(authenticate, c.String())
	}
	g.write(w, p)
}

// write sends a problem, uncacheable (section 11.10).
func (g *Gate) write(w http.ResponseWriter, p *Problem) {
	w.Header().Set("Cache-Control", "no-store")
	p.write(w)
}

// plain is a problem of no payment type.
func plain(status int, detail string) *Problem {
	return &Problem{Type: "about:blank", Title: http.StatusText(status), Status: status, Detail: detail}
}

// refuse answers a host callback's error: 401, 403, or 500.
func (g *Gate) refuse(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrUnauthorized):
		g.write(w, plain(http.StatusUnauthorized, "Authentication is required."))
	case errors.Is(err, ErrDenied):
		g.write(w, plain(http.StatusForbidden, "The request is not allowed."))
	default:
		g.fail(w, err)
	}
}

// fail answers 500 and reports the cause.
func (g *Gate) fail(w http.ResponseWriter, err error) {
	g.c.Report(err)
	g.write(w, Refuse(Internal, "The payment could not be processed."))
}

// answer sends a *Problem: with a fresh challenge when it is a 402 (section 4.2). Any
// other error is an outcome not known.
func (g *Gate) answer(w http.ResponseWriter, err error, again func(*Problem)) {
	var p *Problem
	switch {
	case !errors.As(err, &p):
		g.fail(w, err)
	case p.Status == http.StatusPaymentRequired:
		again(p)
	default:
		g.write(w, p)
	}
}

// known returns the secret a challenge's id is bound under, current or retired.
func (g *Gate) known(c Challenge) (secret []byte, ok bool) {
	for _, s := range append([][]byte{g.c.Secret}, g.c.Previous...) {
		if bound(s, c) {
			return s, true
		}
	}
	return nil, false
}

func (g *Gate) method(name string) Method {
	for _, m := range g.c.Methods {
		if m.Name() == name {
			return m
		}
	}
	return nil
}

// validate checks an echoed challenge against what this gate issued and the request made
// now: binding, realm, route, caller, field, expiry, method, terms and body.
func (g *Gate) validate(r *http.Request, caller Caller, q Quote, h held, body []byte) (Method, []byte, *Problem) {
	if h.err != nil {
		return nil, nil, Refuse(Malformed, h.err.Error())
	}
	c := h.cred.Challenge
	if err := c.Valid(); err != nil {
		return nil, nil, Refuse(Malformed, err.Error())
	}
	secret, ok := g.known(c)
	if c.Realm != g.c.Realm || c.Intent != Charge || !ok {
		return nil, nil, Refuse(Invalid, "The challenge is unknown.")
	}
	members, _ := Opaque(c.Opaque)
	if members["route"] != route(r) || !hmac.Equal([]byte(members["caller"]), []byte(tag(secret, caller))) {
		return nil, nil, Refuse(Invalid, "The challenge was issued for another request or caller.")
	}
	if c.Field() != h.field {
		return nil, nil, Refuse(Malformed, "The credential must be sent in "+c.Field()+".")
	}
	if at, _ := c.Expiry(); !g.c.Now().Before(at) {
		return nil, nil, Refuse(Expired, "The challenge has expired.")
	}
	m := g.method(c.Method)
	if m == nil {
		return nil, nil, Refuse(Unsupported, "The payment method is not accepted.")
	}
	request, t, err := termsOf(c.Request)
	if err != nil || !isAmount(t.Amount) || t.Currency != q.Currency || t.ExternalID != q.External {
		return nil, nil, Refuse(Invalid, "The challenge was issued for another request.")
	}
	switch cmp := cmpAmount(t.Amount, q.Amount); {
	case cmp < 0:
		return nil, nil, Refuse(Insufficient, "The amount is less than the price of this request.")
	case cmp > 0:
		return nil, nil, Refuse(Invalid, "The price of this request has changed.")
	}
	if c.Digest != "" && (g.c.Digest == 0 || !digestMatches(c.Digest, body)) || c.Digest == "" && g.c.Digest > 0 && len(body) > 0 {
		return nil, nil, Refuse(Failed, "The request body is not the one the challenge was issued for.")
	}
	return m, request, nil
}

// redeem settles a credential once and serves the resource: the credential is taken, then
// verified, then settled, then the resource runs with the receipt added (sections 11.3,
// 11.5). A refusal before settlement frees the credential; a refusal after it, or an
// error that is not a *Problem, leaves it spent.
func (g *Gate) redeem(w http.ResponseWriter, r *http.Request, next http.Handler, caller Caller, q Quote, h held, body []byte) {
	ctx := r.Context()
	again := func(p *Problem) { g.challenge(w, r, caller, q, body, p) }
	m, request, p := g.validate(r, caller, q, h, body)
	if p != nil {
		g.answer(w, p, again)
		return
	}
	c := h.cred.Challenge
	at, _ := c.Expiry()
	until := at.Add(g.c.Lifetime)
	took, err := g.c.Store.Take(ctx, "claim/"+c.ID, until)
	if err != nil {
		g.fail(w, err)
		return
	}
	key := strings.TrimSpace(r.Header.Get(idempotency))
	if !took {
		g.resend(w, r, next, c, key, h.field, until, again)
		return
	}
	claim := Claim{Caller: caller, Challenge: c, Request: request, Source: h.cred.Source, Payload: h.cred.Payload, Until: until}
	if err := m.Verify(ctx, claim); err != nil {
		_ = g.c.Store.Drop(context.WithoutCancel(ctx), "claim/"+c.ID)
		g.answer(w, err, again)
		return
	}
	receipt, err := m.Settle(ctx, claim)
	if err != nil {
		g.answer(w, err, again)
		return
	}
	if receipt == nil {
		g.fail(w, fmt.Errorf("%s: settled without a receipt for challenge %s", m.Name(), c.ID))
		return
	}
	receipt.Status = "success"
	if receipt.Method == "" {
		receipt.Method = m.Name()
	}
	if receipt.Timestamp == "" {
		receipt.Timestamp = g.c.Now().UTC().Format(time.RFC3339)
	}

	// The payment has settled: the resource is served whatever the store says.
	free := context.WithoutCancel(ctx)
	rec := record{Key: key, Receipt: receipt.String()}
	if _, err := g.c.Store.Take(free, "deliver/"+c.ID, until); err != nil {
		g.c.Report(fmt.Errorf("challenge %s settled and its delivery was not recorded: %w", c.ID, err))
	}
	if b, err := json.Marshal(rec); err != nil {
		g.c.Report(err)
	} else if err := g.c.Store.Save(free, "done/"+c.ID, b, until); err != nil {
		g.c.Report(fmt.Errorf("challenge %s settled and its receipt was not recorded: %w", c.ID, err))
	}
	g.deliver(w, r, next, c.ID, h.field, rec.Receipt)
}

// record is what a Gate keeps of a settled payment: its receipt and the Idempotency-Key of
// the request that paid.
type record struct {
	Key     string `json:"key,omitempty"`
	Receipt string `json:"receipt"`
}

// resend answers a credential whose challenge is already taken. A retry carrying the
// Idempotency-Key of the request that paid, after that request's response failed, runs the
// resource again without a second charge (section 11.4); any other is a spent credential.
func (g *Gate) resend(w http.ResponseWriter, r *http.Request, next http.Handler, c Challenge, key, field string, until time.Time, again func(*Problem)) {
	ctx := r.Context()
	spent := func() { again(Refuse(Invalid, "The challenge is already used.")) }
	b, found, err := g.c.Store.Load(ctx, "done/"+c.ID)
	if err != nil {
		g.fail(w, err)
		return
	}
	var rec record
	if !found || key == "" || json.Unmarshal(b, &rec) != nil || rec.Key != key {
		spent()
		return
	}
	took, err := g.c.Store.Take(ctx, "deliver/"+c.ID, until)
	if err != nil {
		g.fail(w, err)
		return
	}
	if !took {
		spent()
		return
	}
	g.deliver(w, r, next, c.ID, field, rec.Receipt)
}

// deliver runs the resource with the receipt added to a 2xx response (section 5.3). A
// panic or a non-2xx response frees the delivery for a retry: the payment has settled and
// the client was given nothing for it.
func (g *Gate) deliver(w http.ResponseWriter, r *http.Request, next http.Handler, id, field, receipt string) {
	t := &tee{ResponseWriter: w, receipt: receipt, field: field}
	free := context.WithoutCancel(r.Context())
	returned := false
	defer func() {
		if !returned || (t.code != 0 && !t.paid()) {
			_ = g.c.Store.Drop(free, "deliver/"+id)
		}
	}()
	next.ServeHTTP(t, withoutPayment(r))
	if t.code == 0 && !t.hijacked {
		t.WriteHeader(http.StatusOK) // returned without writing: the answer is 200 and carries the receipt
	}
	returned = true
}

// withoutPayment is the request without its Payment credentials, so the resource never
// holds one (section 11.2.1).
func withoutPayment(r *http.Request) *http.Request {
	out := r.Clone(r.Context())
	out.Header.Del(alternate)
	var keep []string
	for _, v := range out.Header.Values(authorization) {
		if !isPayment(v) {
			keep = append(keep, v)
		}
	}
	if len(keep) == 0 {
		out.Header.Del(authorization)
	} else {
		out.Header[authorization] = keep
	}
	return out
}

// private makes a response uncacheable by shared caches (section 11.10), leaving no-store
// and private as the resource set them.
func private(h http.Header) {
	if v := strings.ToLower(h.Get("Cache-Control")); !strings.Contains(v, "no-store") && !strings.Contains(v, "private") {
		h.Set("Cache-Control", "private")
	}
}

// tee is the writer a paid resource writes to: it adds the receipt to a 2xx response and
// notes what the resource did.
type tee struct {
	http.ResponseWriter
	receipt  string
	field    string
	code     int
	hijacked bool
}

func (t *tee) paid() bool { return t.code >= 200 && t.code < 300 }

func (t *tee) WriteHeader(code int) {
	if code >= 100 && code < 200 && code != http.StatusSwitchingProtocols {
		t.ResponseWriter.WriteHeader(code)
		return
	}
	if t.code != 0 {
		return
	}
	t.code = code
	if t.paid() {
		t.Header().Set(receipted, t.receipt)
		private(t.Header())
	} else if t.field == alternate {
		private(t.Header()) // Authorization's shared-cache rule does not cover this field (section 11.10)
	}
	t.ResponseWriter.WriteHeader(code)
}

func (t *tee) Write(b []byte) (int, error) {
	if t.code == 0 {
		t.WriteHeader(http.StatusOK)
	}
	return t.ResponseWriter.Write(b)
}

func (t *tee) Flush() {
	if t.code == 0 {
		t.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(t.ResponseWriter).Flush()
}

func (t *tee) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	c, rw, err := http.NewResponseController(t.ResponseWriter).Hijack()
	if err == nil {
		t.hijacked = true
	}
	return c, rw, err
}

func (t *tee) Unwrap() http.ResponseWriter { return t.ResponseWriter }
