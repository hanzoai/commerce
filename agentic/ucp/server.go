// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Config is what a host states to serve the business side of UCP.
type Config struct {
	// Profile is the business profile (NewProfile). Its REST endpoint fixes the
	// path the server answers under; its payment handlers are the ones every
	// checkout response advertises.
	Profile *Profile
	// Backend sells.
	Backend Backend
	// Identify says who a request acts for: the buyer and the merchant. It runs
	// before the platform is authenticated, so that a caller who is not
	// authenticated costs no profile fetch, and it never sees the verified
	// platform. It returns ErrUnauthorized when the request is not authenticated
	// and ErrForbidden when it may not act. The platform that signed the request
	// reaches the Backend as Caller.Platform; Trusted limits which platforms are
	// served at all.
	Identify func(*http.Request) (Caller, error)
	// Profiles resolves platform profiles from UCP-Agent (NewFetcher).
	Profiles Profiles
	// Trusted, when set, is the registry of pre-approved platforms ("Fetching"):
	// it is asked about the profile URL in UCP-Agent, and a platform it refuses
	// is answered profile_not_trusted (403) before its profile is fetched.
	Trusted func(profile string) bool
	// Verifier verifies request signatures. When set, a request that carries a
	// signature must verify; Require makes a signature mandatory. Without a
	// Verifier no signature is checked and the platform is authenticated by
	// whatever Identify does. See Verifier.Window for how long a signature
	// stays valid.
	Verifier *Verifier
	Require  bool
	// Signer signs responses (signatures, "REST Response Signing"); the
	// specification recommends it for complete_checkout and leaves the rest
	// optional, so every response with a body is signed.
	Signer *Signer
	// Store keeps idempotency records. Required: how many keys it may hold is the
	// host's to choose.
	Store Store
	// Links are shown with every checkout that carries none of its own: privacy
	// policy, terms of service and so on, which the business must provide for
	// legal compliance (shopping/checkout, "Link").
	Links []Link
	// Storefront is the https URL negotiation failures hand a buyer to
	// ("The continue_url Field").
	Storefront string
	// MaxBody bounds a request body; default 1 MiB.
	MaxBody int64
	// Webhooks, when set with an Orders backend and the Order capability
	// advertised, announces each order a completion places to the platforms that
	// negotiated the capability.
	Webhooks *Webhooks
	// Fault is told of failures the server answers with 500, and of webhook
	// deliveries that were not acknowledged, for the host's log.
	Fault func(op string, err error)
}

// Server serves the REST binding of the Checkout capability, and Get Order when
// the profile advertises the Order capability, under the path of the profile's
// REST endpoint. Mount it at that path; mount Profile() at /.well-known/ucp.
type Server struct {
	cfg     Config
	base    string
	profile []byte
	etag    string
	orders  Orders
}

// New returns a Server. It validates the configuration: the profile must be a
// valid business profile with a REST binding of dev.ucp.shopping.
func New(cfg Config) (*Server, error) {
	if cfg.Profile == nil || cfg.Backend == nil || cfg.Identify == nil || cfg.Profiles == nil {
		return nil, errors.New("ucp: Profile, Backend, Identify and Profiles are required")
	}
	if cfg.Require && cfg.Verifier == nil {
		return nil, errors.New("ucp: Require needs a Verifier")
	}
	if err := cfg.Profile.Validate(Business); err != nil {
		return nil, err
	}
	var endpoint string
	for _, s := range cfg.Profile.UCP.Services[ServiceShopping] {
		if s.Transport == TransportREST {
			endpoint = s.Endpoint
		}
	}
	if endpoint == "" {
		return nil, errors.New("ucp: the profile has no REST binding of " + ServiceShopping)
	}
	if _, ok := cfg.Profile.UCP.Capabilities[CapCheckout]; !ok {
		return nil, errors.New("ucp: the profile does not advertise " + CapCheckout)
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	if cfg.Storefront != "" {
		if err := checkURL(cfg.Storefront); err != nil {
			return nil, fmt.Errorf("ucp: Storefront: %w", err)
		}
	}
	if cfg.Store == nil {
		return nil, errors.New("ucp: Store is required: NewMemory for one process, Idempotency for replicas that share a datastore")
	}
	if cfg.MaxBody <= 0 {
		cfg.MaxBody = 1 << 20
	}
	s := &Server{cfg: cfg, base: strings.TrimSuffix(u.Path, "/")}
	if _, ok := cfg.Profile.UCP.Capabilities[CapOrder]; ok {
		o, ok := cfg.Backend.(Orders)
		if !ok {
			return nil, errors.New("ucp: the profile advertises " + CapOrder + " and the Backend does not serve orders")
		}
		s.orders = o
	}
	b, err := json.Marshal(cfg.Profile)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(b)
	s.profile, s.etag = b, `"`+hex.EncodeToString(sum[:16])+`"`
	return s, nil
}

// Profile serves the business profile at /.well-known/ucp (overview,
// "Hosting"): over HTTPS by the host, without redirecting, with Cache-Control
// public and a max-age of at least 60 seconds, and an ETag so consumers can
// revalidate.
func (s *Server) Profile() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h := w.Header()
		h.Set("Content-Type", "application/json")
		h.Set("Cache-Control", "public, max-age=60")
		h.Set("ETag", s.etag)
		if match := r.Header.Get("If-None-Match"); match != "" && etagMatch(match, s.etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		h.Set("Content-Length", strconv.Itoa(len(s.profile)))
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			w.Write(s.profile)
		}
	})
}

func etagMatch(header, etag string) bool {
	for _, t := range strings.Split(header, ",") {
		t = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(t), "W/"))
		if t == "*" || t == etag {
			return true
		}
	}
	return false
}

// reply is a response before it is written.
type reply struct {
	status int
	body   []byte
	header http.Header
}

func jsonReply(status int, v any) reply {
	b, err := json.Marshal(v)
	if err != nil {
		return reply{status: http.StatusInternalServerError, body: []byte(`{"code":"internal","content":"the response could not be encoded"}`)}
	}
	return reply{status: status, body: b}
}

func errorReply(e *Error) reply {
	r := jsonReply(e.Status, e)
	r.header = http.Header{}
	if e.RetryAfter > 0 {
		r.header.Set("Retry-After", strconv.Itoa(e.RetryAfter))
	}
	if e.Status == http.StatusUnauthorized && !strings.HasPrefix(e.Code, "signature_") && e.Code != CodeKeyNotFound {
		r.header.Set("WWW-Authenticate", `Bearer realm="ucp"`)
	}
	return r
}

type op int

const (
	opCreate op = iota + 1
	opRead
	opUpdate
	opComplete
	opCancel
	opOrder
)

func (o op) String() string {
	return [...]string{"", "create_checkout", "get_checkout", "update_checkout", "complete_checkout", "cancel_checkout", "get_order"}[o]
}

func (o op) mutates() bool { return o == opCreate || o == opUpdate || o == opComplete || o == opCancel }

// route maps a request to an operation and a resource id.
func (s *Server) route(r *http.Request) (o op, id string, e *Error) {
	p, ok := strings.CutPrefix(r.URL.Path, s.base)
	if !ok || !strings.HasPrefix(p, "/") {
		return 0, "", status(http.StatusNotFound, "not_found", "no such resource")
	}
	parts := strings.Split(strings.Trim(p, "/"), "/")
	allow := func(m string, o op, id string) (op, string, *Error) {
		if r.Method != m {
			return 0, "", status(http.StatusMethodNotAllowed, "method_not_allowed", "use %s", m)
		}
		return o, id, nil
	}
	switch {
	case len(parts) == 1 && parts[0] == "checkout-sessions":
		return allow(http.MethodPost, opCreate, "")
	case len(parts) == 2 && parts[0] == "checkout-sessions" && parts[1] != "":
		if r.Method == http.MethodPut {
			return opUpdate, parts[1], nil
		}
		return allow(http.MethodGet, opRead, parts[1])
	case len(parts) == 3 && parts[0] == "checkout-sessions" && parts[1] != "" && parts[2] == "complete":
		return allow(http.MethodPost, opComplete, parts[1])
	case len(parts) == 3 && parts[0] == "checkout-sessions" && parts[1] != "" && parts[2] == "cancel":
		return allow(http.MethodPost, opCancel, parts[1])
	case s.orders != nil && len(parts) == 2 && parts[0] == "orders" && parts[1] != "":
		return allow(http.MethodGet, opOrder, parts[1])
	}
	return 0, "", status(http.StatusNotFound, "not_found", "no such resource")
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rep := s.handle(r)
	s.write(w, r, rep)
}

func (s *Server) write(w http.ResponseWriter, r *http.Request, rep reply) {
	h := w.Header()
	for k, v := range rep.header {
		h[k] = v
	}
	if id := r.Header.Get(HeaderRequestID); id != "" {
		h.Set(HeaderRequestID, id)
	}
	if len(rep.body) > 0 {
		h.Set("Content-Type", "application/json")
		if s.cfg.Signer != nil {
			if err := s.cfg.Signer.SignResponse(h, rep.status, rep.body, Options{}); err != nil {
				s.fault("sign", err)
				rep = errorReply(status(http.StatusInternalServerError, "internal", "the response could not be signed"))
				for k := range h {
					delete(h, k)
				}
				h.Set("Content-Type", "application/json")
			}
		}
	}
	h.Set("Content-Length", strconv.Itoa(len(rep.body)))
	w.WriteHeader(rep.status)
	w.Write(rep.body)
}

func (s *Server) fault(op string, err error) {
	if s.cfg.Fault != nil {
		s.cfg.Fault(op, err)
	}
}

// request is what the pipeline knows once a request is admitted.
type request struct {
	r      *http.Request
	op     op
	id     string
	body   []byte
	who    Caller
	active *Active
	peer   *Profile
}

func (s *Server) handle(r *http.Request) reply {
	o, id, e := s.route(r)
	if e != nil {
		return errorReply(e)
	}
	body, e := s.body(r, o)
	if e != nil {
		return errorReply(e)
	}
	who, err := s.cfg.Identify(r)
	switch {
	case errors.Is(err, ErrForbidden):
		return errorReply(status(http.StatusForbidden, "forbidden", "not permitted"))
	case err != nil:
		return errorReply(status(http.StatusUnauthorized, "unauthorized", "authentication is required"))
	}

	agent, e := ParseAgent(r.Header.Get(HeaderAgent))
	if e != nil {
		return errorReply(e)
	}
	if s.cfg.Trusted != nil && !s.cfg.Trusted(agent) {
		return errorReply(fail(CodeProfileNotTrusted, "the platform profile is not in this business's registry of pre-approved platforms"))
	}
	if s.cfg.Verifier != nil && (s.cfg.Require || r.Header.Get("Signature") != "" || r.Header.Get("Signature-Input") != "") {
		signed, err := s.cfg.Verifier.Request(r.Context(), r, body)
		if err != nil {
			return s.protocol(err)
		}
		who.Platform = signed.Identity
	}
	peer, err := s.cfg.Profiles.Platform(r.Context(), agent, false)
	if err != nil {
		return s.protocol(err)
	}
	req := &request{r: r, op: o, id: id, body: body, who: who, peer: peer}
	if !s.cfg.Profile.UCP.Supports(peer.UCP.Version) {
		e := fail(CodeVersionUnsupported, "Protocol version %s is not supported. This business supports version %s.", peer.UCP.Version, s.cfg.Profile.UCP.Version)
		e.ContinueURL = s.cfg.Storefront
		return errorReply(e)
	}
	req.active = Negotiate(&s.cfg.Profile.UCP, &peer.UCP, nil)
	root := CapCheckout
	if o == opOrder {
		root = CapOrder
	}
	if !req.active.Has(root) {
		return jsonReply(http.StatusOK, s.errorResponse(req, root, "", Err(CodeCapabilitiesIncompatible, Unrecoverable, "", "No compatible capabilities in the intersection")))
	}
	if !o.mutates() {
		return s.dispatch(req)
	}
	return s.idempotent(req)
}

// protocol answers an error from signature verification or discovery.
func (s *Server) protocol(err error) reply {
	var e *Error
	if errors.As(err, &e) {
		if e.ContinueURL == "" && (e.Code == CodeVersionUnsupported || strings.HasPrefix(e.Code, "profile_")) {
			c := *e
			c.ContinueURL = s.cfg.Storefront
			e = &c
		}
		return errorReply(e)
	}
	s.fault("protocol", err)
	return errorReply(status(http.StatusInternalServerError, "internal", "unexpected error"))
}

// body reads the request body, bounded, and checks its media type.
func (s *Server) body(r *http.Request, o op) ([]byte, *Error) {
	b, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, s.cfg.MaxBody))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return nil, status(http.StatusRequestEntityTooLarge, "request_too_large", "the request body is larger than %d bytes", s.cfg.MaxBody)
		}
		return nil, status(http.StatusBadRequest, "invalid_request", "the request body could not be read")
	}
	if len(b) > 0 {
		mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mt != "application/json" {
			return nil, status(http.StatusBadRequest, "invalid_request", "the request body must be application/json")
		}
	}
	if len(bytes.TrimSpace(b)) == 0 && (o == opCreate || o == opUpdate || o == opComplete) {
		return nil, status(http.StatusBadRequest, "invalid_request", "the request needs a JSON body")
	}
	return b, nil
}

// idempotent runs a state-changing operation under its Idempotency-Key.
func (s *Server) idempotent(q *request) reply {
	key := q.r.Header.Get(HeaderIdempotency)
	if key == "" {
		return errorReply(status(http.StatusBadRequest, "idempotency_key_missing", "the Idempotency-Key header is required"))
	}
	if len(key) < minKeyLength || len(key) > 255 || strings.IndexFunc(key, func(c rune) bool { return c < 0x21 || c > 0x7e }) >= 0 {
		return errorReply(status(http.StatusBadRequest, "idempotency_key_invalid",
			"the Idempotency-Key must be 22 to 255 visible characters carrying at least 128 bits of entropy, such as a UUID"))
	}
	sum := sha256.Sum256(q.body)
	digest := hex.EncodeToString(sum[:])
	scope := q.who.key() + "\x00" + q.op.String() + "\x00" + q.id
	ctx := context.WithoutCancel(q.r.Context())
	seen, err := s.cfg.Store.Begin(ctx, scope, key, digest)
	if err != nil {
		s.fault("idempotency", err)
		e := status(http.StatusServiceUnavailable, "unavailable", "the request cannot be made safely repeatable now")
		e.RetryAfter = 1
		return errorReply(e)
	}
	if seen != nil {
		switch {
		case seen.Digest != digest:
			return errorReply(status(http.StatusConflict, "idempotency_key_reused", "the Idempotency-Key was used with a different request body"))
		case !seen.Done:
			return errorReply(status(http.StatusConflict, "idempotency_in_progress", "a request with this Idempotency-Key is being processed"))
		}
		return reply{status: seen.Response.Status, body: seen.Response.Body}
	}
	rep := s.dispatch(q)
	if rep.status >= 500 {
		_ = s.cfg.Store.Release(ctx, scope, key)
		return rep
	}
	if err := s.cfg.Store.Finish(ctx, scope, key, digest, Stored{Status: rep.status, ContentType: "application/json", Body: rep.body}); err != nil {
		s.fault("idempotency", err)
	}
	return rep
}
