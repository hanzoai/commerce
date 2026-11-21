// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
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

const idempotency = "Idempotency-Key"

// maxChallenge is the size servers keep a challenge under (section 9.4).
const maxChallenge = 8192

// Config is everything a Gate takes from its host. Nothing environmental has a default
// that decides money: the price, the secret, the store and the methods are the host's.
type Config struct {
	// Realm is the protection space the challenges name (section 5.1.1). A credential
	// echoing another realm is refused.
	Realm string
	// Secret binds challenge ids with HMAC-SHA256 (section 5.1.2.1.1). At least 32
	// bytes, the hash's output size (RFC 2104 section 3); it stays on the server
	// (section 11.2.2).
	Secret []byte
	// Previous are secrets retired by a rotation, still accepted so that challenges
	// issued under them stay good until they expire (section 11.2.2).
	Previous [][]byte
	// Store holds the spent credentials; it must be shared by every process serving the
	// resource.
	Store Store
	// Methods are the payment methods offered, in the server's order of preference.
	Methods []Method
	// Price is what the request must pay, or nil when it pays nothing. It may return
	// ErrUnauthorized or ErrDenied.
	Price func(*http.Request) (*Quote, error)
	// Caller is who the request is for. Nil means anonymous: no organization and no
	// subject. It may return ErrUnauthorized or ErrDenied.
	Caller func(*http.Request) (Caller, error)
	// Secure says the request arrived over TLS (section 11.2). The default is
	// r.TLS != nil; a host behind a TLS-terminating proxy supplies the check it trusts.
	Secure func(*http.Request) bool
	// Lifetime is how long a challenge is good. The draft's examples and the method
	// documentation use five minutes, the default.
	Lifetime time.Duration
	// Alternate has challenges select Payment-Authorization for the credential, which
	// leaves Authorization to the host's own authentication (section 4.4.3).
	Alternate bool
	// Digest, when positive, binds the request body to the challenge (section 5.1.3):
	// bodies up to this many bytes are buffered, hashed with SHA-256 and checked on the
	// paid request. A larger body is refused with 413.
	Digest int64
	// Replay, when positive, keeps a paid response of up to this many bytes in the Store,
	// so a retry with the same Idempotency-Key and credential is answered with it instead
	// of paying or running the resource again (section 11.4). The Store then holds paid
	// responses, which may be private. Zero keeps none; a retry after a response that
	// failed still runs the resource without charging again.
	Replay int64
	// Limit is the largest credential field value accepted, in bytes. The draft requires
	// at least 4096 (section 9.4); the default, 16384, holds an 8 KiB challenge echoed in
	// base64url and a payload.
	Limit int
	// Report is told of what cannot be answered to the client: a payment that may have
	// settled with no record of it, or one whose outcome is unknown. Nil drops them.
	Report func(error)
	// Now is the clock. Nil means time.Now.
	Now func() time.Time
}

// Gate is net/http middleware that puts a resource behind the Payment scheme.
type Gate struct {
	c Config
}

// New checks a configuration and returns the Gate for it.
func New(c Config) (*Gate, error) {
	switch {
	case c.Realm == "":
		return nil, errors.New("mpp: realm is required")
	case len(c.Secret) < 32:
		return nil, errors.New("mpp: secret must be at least 32 bytes")
	case c.Store == nil:
		return nil, errors.New("mpp: store is required")
	case len(c.Methods) == 0:
		return nil, errors.New("mpp: at least one method is required")
	case c.Price == nil:
		return nil, errors.New("mpp: price is required")
	case c.Lifetime < 0 || c.Digest < 0 || c.Replay < 0 || c.Limit < 0:
		return nil, errors.New("mpp: lifetime, digest, replay and limit cannot be negative")
	case c.Limit != 0 && c.Limit < 4096:
		return nil, errors.New("mpp: a credential limit under 4096 bytes breaks section 9.4")
	}
	for _, s := range c.Previous {
		if len(s) < 32 {
			return nil, errors.New("mpp: a previous secret is shorter than 32 bytes")
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
	if c.Lifetime == 0 {
		c.Lifetime = 5 * time.Minute
	}
	if c.Limit == 0 {
		c.Limit = 16384
	}
	if c.Secure == nil {
		c.Secure = func(r *http.Request) bool { return r.TLS != nil }
	}
	if c.Caller == nil {
		c.Caller = func(*http.Request) (Caller, error) { return Caller{}, nil }
	}
	if c.Report == nil {
		c.Report = func(error) {}
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return &Gate{c}, nil
}

// Wrap returns a handler that serves next only for a request that has paid.
//
// The order is the draft's: the host's authentication and policy first (Caller and
// Price), then the credential. A request with none gets a 402 with a challenge for each
// method that offers one, ranked by its Accept-Payment header. A request with one has
// the echoed challenge checked against its binding, its realm, its expiry, its field
// and the price the host now names; the credential is then taken, verified and settled
// exactly once, and only then does next run, with the receipt added to a 2xx response.
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
	creds := credentials(r, g.c.Limit)
	switch len(creds) {
	case 0:
		g.challenge(w, r, caller, *quote, body, nil)
	case 1:
		g.redeem(w, r, next, caller, *quote, creds[0], body)
	default:
		g.write(w, Refuse(Bad, "More than one Payment credential."))
	}
}

// held is a Payment credential found on a request, or why it could not be read.
type held struct {
	cred  *Credential
	field string
	err   error
}

var errLarge = errors.New("credential is larger than the server accepts")

// credentials lists the Payment credentials of a request, in Authorization and
// Payment-Authorization. A field value of another scheme is not one.
func credentials(r *http.Request, limit int) []held {
	var out []held
	for _, field := range []string{authorize, alternate} {
		for _, v := range r.Header.Values(field) {
			scheme, _, _ := strings.Cut(strings.TrimSpace(v), " ")
			if !strings.EqualFold(scheme, Scheme) {
				continue
			}
			if len(v) > limit {
				out = append(out, held{field: field, err: errLarge})
				continue
			}
			c, err := ParseCredential(v)
			out = append(out, held{c, field, err})
		}
	}
	return out
}

// body buffers the request body to the digest limit and puts it back, so the digest
// binds the bytes the resource will read. It does nothing unless digests are on.
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
		p := Refuse(Bad, "The request body is larger than the server binds to a payment.")
		p.Status = http.StatusRequestEntityTooLarge
		return nil, p
	}
	r.Body = io.NopCloser(bytes.NewReader(b))
	return b, nil
}

// issue makes a challenge for each method that offers one for the quote.
func (g *Gate) issue(ctx context.Context, caller Caller, q Quote, body []byte) ([]Challenge, error) {
	expires := g.c.Now().UTC().Add(g.c.Lifetime).Format(time.RFC3339)
	var out []Challenge
	for _, m := range g.c.Methods {
		p, err := m.Propose(ctx, caller, q)
		if errors.Is(err, ErrSkip) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", m.Name(), err)
		}
		req, err := Encode(p.Request)
		if err != nil {
			return nil, fmt.Errorf("%s: request: %w", m.Name(), err)
		}
		c := Challenge{Realm: g.c.Realm, Method: m.Name(), Intent: Charge, Request: req, Expires: expires, Description: q.Description}
		if g.c.Alternate {
			c.Header = alternate
		}
		// The binding is deterministic, so two clients challenged for the same request in
		// the same second would share one id and the second to pay would find it spent. A
		// random member of opaque, which the binding covers and clients return unchanged,
		// makes every challenge's id unique (section 5.1.1).
		opaque := map[string]string{}
		for k, v := range p.Opaque {
			opaque[k] = v
		}
		if _, taken := opaque["nonce"]; taken {
			return nil, fmt.Errorf("%s: opaque member nonce belongs to the gate", m.Name())
		}
		var nonce [16]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return nil, err
		}
		opaque["nonce"] = base64.RawURLEncoding.EncodeToString(nonce[:])
		if c.Opaque, err = Encode(opaque); err != nil {
			return nil, fmt.Errorf("%s: opaque: %w", m.Name(), err)
		}
		if len(body) > 0 {
			c.Digest = digestOf(body)
		}
		c.ID = Bind(g.c.Secret, c)
		if err := c.Valid(); err != nil {
			return nil, fmt.Errorf("%s: %w", m.Name(), err)
		}
		if len(c.String()) > maxChallenge {
			return nil, fmt.Errorf("%s: challenge is larger than %d bytes", m.Name(), maxChallenge)
		}
		// The request must ask what the host priced: a method cannot choose the amount.
		var t Terms
		b, _ := raw.DecodeString(req)
		if err := json.Unmarshal(b, &t); err != nil || t.Amount != q.Amount || t.Currency != q.Currency || t.ExternalID != q.External {
			return nil, fmt.Errorf("%s: request does not carry the quoted amount, currency and external id", m.Name())
		}
		out = append(out, c)
	}
	return out, nil
}

// preferred orders challenges by the request's Accept-Payment header (section 7.4). A
// header that is malformed, or that matches none of them, is ignored.
func preferred(r *http.Request, cs []Challenge) []Challenge {
	ranges, err := ParseAccept(r.Header.Values(accepting))
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

// challenge answers 402 with a fresh challenge for each method and the problem given,
// payment-required when none (section 4.2).
func (g *Gate) challenge(w http.ResponseWriter, r *http.Request, caller Caller, q Quote, body []byte, p *Problem) {
	cs, err := g.issue(r.Context(), caller, q, body)
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

// write sends a problem the response of which must not be cached (section 11.10).
func (g *Gate) write(w http.ResponseWriter, p *Problem) {
	w.Header().Set("Cache-Control", "no-store")
	p.write(w)
}

// plain is a problem of no payment type.
func plain(status int, detail string) *Problem {
	return &Problem{Type: "about:blank", Title: http.StatusText(status), Status: status, Detail: detail}
}

// refuse answers what a host's callback said: 401, 403 or, for any other error, 500.
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

// fail answers 500 and tells the host why; the client is told nothing of the cause.
func (g *Gate) fail(w http.ResponseWriter, err error) {
	g.c.Report(err)
	g.write(w, Refuse(Internal, "The payment could not be processed."))
}

// answer sends what a method said: a refusal of 402 gets a fresh challenge, another
// refusal is sent as it is, and any other error is an outcome not known.
func (g *Gate) answer(w http.ResponseWriter, err error, again func(*Problem)) {
	var p *Problem
	if !errors.As(err, &p) {
		g.fail(w, err)
		return
	}
	if p.Status == http.StatusPaymentRequired {
		again(p)
		return
	}
	g.write(w, p)
}

func (g *Gate) known(c Challenge) bool {
	if bound(g.c.Secret, c) {
		return true
	}
	for _, s := range g.c.Previous {
		if bound(s, c) {
			return true
		}
	}
	return false
}

func (g *Gate) method(name string) Method {
	for _, m := range g.c.Methods {
		if m.Name() == name {
			return m
		}
	}
	return nil
}

// idem is the request's Idempotency-Key when it is a usable one: visible ASCII, at most
// 255 bytes.
func idem(r *http.Request) string {
	k := strings.TrimSpace(r.Header.Get(idempotency))
	if len(k) > 255 {
		return ""
	}
	for i := 0; i < len(k); i++ {
		if k[i] < 0x21 || k[i] > 0x7e {
			return ""
		}
	}
	return k
}

// redeem checks a credential and, when it holds, settles it and serves the resource.
func (g *Gate) redeem(w http.ResponseWriter, r *http.Request, next http.Handler, caller Caller, q Quote, h held, body []byte) {
	ctx := r.Context()
	again := func(p *Problem) { g.challenge(w, r, caller, q, body, p) }
	if h.err != nil {
		again(Refuse(Malformed, h.err.Error()))
		return
	}
	c := h.cred.Challenge
	if err := c.Valid(); err != nil {
		again(Refuse(Malformed, err.Error()))
		return
	}
	if c.Realm != g.c.Realm || c.Intent != Charge || !g.known(c) {
		again(Refuse(Invalid, "The challenge is unknown."))
		return
	}
	if c.Field() != h.field {
		again(Refuse(Malformed, "The credential must be sent in "+c.Field()+"."))
		return
	}
	at, _ := c.Expiry()
	now := g.c.Now()
	if !now.Before(at) {
		again(Refuse(Expired, "The challenge has expired."))
		return
	}
	m := g.method(c.Method)
	if m == nil {
		g.write(w, Refuse(Unsupported, "The payment method is not accepted."))
		return
	}
	request, _ := raw.DecodeString(c.Request)
	var t Terms
	if err := json.Unmarshal(request, &t); err != nil || !isAmount(t.Amount) || t.Currency != q.Currency || t.ExternalID != q.External {
		again(Refuse(Invalid, "The challenge was issued for another request."))
		return
	}
	switch cmp := cmpAmount(t.Amount, q.Amount); {
	case cmp < 0:
		again(Refuse(Insufficient, "The amount is less than the price of this request."))
		return
	case cmp > 0:
		again(Refuse(Invalid, "The price of this request has changed."))
		return
	}
	if c.Digest != "" && (g.c.Digest == 0 || !digestMatches(c.Digest, body)) ||
		c.Digest == "" && g.c.Digest > 0 && len(body) > 0 {
		again(Refuse(Failed, "The request body is not the one the challenge was issued for."))
		return
	}

	until := at.Add(g.c.Lifetime)
	took, err := g.c.Store.Take(ctx, "claim/"+c.ID, until)
	if err != nil {
		g.fail(w, err)
		return
	}
	key := idem(r)
	if !took {
		g.resend(w, r, next, c, key, h.field, until, again)
		return
	}

	claim := Claim{Caller: caller, Challenge: c, Request: request, Source: h.cred.Source, Payload: h.cred.Payload, Key: key, At: now, Until: until}
	if err := m.Verify(ctx, claim); err != nil {
		// Nothing was attempted: the credential may be sent again corrected.
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

	// The money has moved: from here the resource is served whatever the store says.
	free := context.WithoutCancel(ctx)
	if _, err := g.c.Store.Take(free, "deliver/"+c.ID, until); err != nil {
		g.c.Report(fmt.Errorf("challenge %s settled and its delivery was not recorded: %w", c.ID, err))
	}
	rec := record{Key: key, Receipt: receipt.String()}
	if err := g.save(free, c.ID, rec, until); err != nil {
		g.c.Report(fmt.Errorf("challenge %s settled and its receipt was not recorded: %w", c.ID, err))
	}
	g.deliver(w, r, next, c.ID, h.field, rec, until)
}

// record is what a Gate keeps of a settled payment: the receipt, the Idempotency-Key the
// paid request carried and, when replay is on, the response it was answered with.
type record struct {
	Key     string      `json:"key,omitempty"`
	Receipt string      `json:"receipt"`
	Status  int         `json:"status,omitempty"`
	Header  http.Header `json:"header,omitempty"`
	Body    []byte      `json:"body,omitempty"`
}

func (g *Gate) save(ctx context.Context, id string, rec record, until time.Time) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return g.c.Store.Save(ctx, "done/"+id, b, until)
}

// resend answers a credential whose challenge is already taken. The one case it serves
// is a retry of a settled payment carrying the Idempotency-Key the first request did:
// the response kept is sent again, or, when the first attempt failed after settlement,
// the resource runs again without a second charge (section 11.4). Anything else is a
// credential spent.
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
	if rec.Status != 0 {
		h := w.Header()
		for k, v := range rec.Header {
			h[k] = v
		}
		h.Set(receipted, rec.Receipt)
		private(h)
		w.WriteHeader(rec.Status)
		_, _ = w.Write(rec.Body)
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
	g.deliver(w, r, next, c.ID, field, rec, until)
}

// deliver runs the resource with the receipt added to a 2xx response. A response that is
// not 2xx frees the delivery for a retry, since the payment has settled and the client
// was given nothing for it.
func (g *Gate) deliver(w http.ResponseWriter, r *http.Request, next http.Handler, id, field string, rec record, until time.Time) {
	t := &tee{ResponseWriter: w, receipt: rec.Receipt, field: field, limit: g.c.Replay}
	free := context.WithoutCancel(r.Context())
	returned := false
	defer func() {
		switch {
		case !returned || (t.code != 0 && !t.paid()):
			// The resource panicked or answered with an error: nothing was delivered.
			_ = g.c.Store.Drop(free, "deliver/"+id)
		case g.c.Replay > 0 && !t.over && t.code != 0:
			rec.Status, rec.Header, rec.Body = t.code, t.sent, t.kept.Bytes()
			if err := g.save(free, id, rec, until); err != nil {
				g.c.Report(fmt.Errorf("challenge %s: the response was not kept for replay: %w", id, err))
			}
		}
	}()
	next.ServeHTTP(t, withoutPayment(r))
	if t.code == 0 && !t.hijacked {
		// The resource returned without writing: net/http answers 200, and the receipt
		// rides on that answer.
		t.WriteHeader(http.StatusOK)
	}
	returned = true
}

// withoutPayment is the request with its Payment credentials removed, so that the
// resource, and whatever logs its requests, never holds one (section 11.2.1). Another
// scheme in Authorization stays.
func withoutPayment(r *http.Request) *http.Request {
	out := r.Clone(r.Context())
	out.Header.Del(alternate)
	var keep []string
	for _, v := range out.Header.Values(authorize) {
		if scheme, _, _ := strings.Cut(strings.TrimSpace(v), " "); !strings.EqualFold(scheme, Scheme) {
			keep = append(keep, v)
		}
	}
	if len(keep) == 0 {
		out.Header.Del(authorize)
	} else {
		out.Header[authorize] = keep
	}
	return out
}

// private makes a response uncacheable by shared caches, leaving no-store and private
// as the resource set them (section 11.10).
func private(h http.Header) {
	v := strings.ToLower(h.Get("Cache-Control"))
	if strings.Contains(v, "no-store") || strings.Contains(v, "private") {
		return
	}
	h.Set("Cache-Control", "private")
}

// tee is the response writer a paid resource writes to: it adds the receipt to a 2xx
// response, keeps the response for replay up to the limit, and passes everything else
// through.
type tee struct {
	http.ResponseWriter
	receipt string
	field   string
	limit   int64
	code    int
	sent    http.Header
	kept    bytes.Buffer
	over    bool
	// hijacked says the resource took the connection over, which answers for it.
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
	h := t.Header()
	if t.paid() {
		h.Set(receipted, t.receipt)
		private(h)
	} else if t.field == alternate {
		private(h) // the shared-cache rule for Authorization does not cover this field
	}
	t.sent = h.Clone()
	t.ResponseWriter.WriteHeader(code)
}

func (t *tee) Write(b []byte) (int, error) {
	if t.code == 0 {
		t.WriteHeader(http.StatusOK)
	}
	if t.limit > 0 && !t.over {
		if int64(t.kept.Len()+len(b)) > t.limit {
			t.over = true
			t.kept.Reset()
		} else {
			t.kept.Write(b)
		}
	}
	return t.ResponseWriter.Write(b)
}

func (t *tee) Flush() {
	if t.code == 0 {
		t.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(t.ResponseWriter).Flush()
}

// Hijack lets a resource take the connection over, and notes that it did.
func (t *tee) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	c, rw, err := http.NewResponseController(t.ResponseWriter).Hijack()
	if err == nil {
		t.hijacked = true
	}
	return c, rw, err
}

func (t *tee) Unwrap() http.ResponseWriter { return t.ResponseWriter }
