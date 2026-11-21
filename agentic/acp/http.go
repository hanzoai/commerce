// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Signed is a request as the agent sent it. The spec says the client signs the canonical JSON
// body with a Timestamp and leaves the algorithm and the keys out of band
// (rfc.agentic_checkout.md §2.1), so the Verifier binds what it needs from here.
type Signed struct {
	Method string
	// Path is the path as requested, before the host strips a prefix.
	Path string
	// Key is the Idempotency-Key of a POST.
	Key       string
	Timestamp string
	Signature string
	Body      []byte
}

// Verifier checks the Signature of a request.
type Verifier interface {
	Verify(ctx context.Context, who Caller, req Signed) error
}

// maxKey is the longest Idempotency-Key (rfc.agentic_checkout.md §6.1).
const maxKey = 255

type route int

const (
	rCreate route = iota + 1
	rSession
	rComplete
	rCancel
)

// API returns the Agentic Checkout REST API (openapi.agentic_checkout.yaml) as an http.Handler
// for the paths below, relative to the API base URL. The host strips its mount prefix and
// bounds the request body (http.MaxBytesHandler).
//
//	POST /checkout_sessions
//	POST /checkout_sessions/{id}
//	GET  /checkout_sessions/{id}
//	POST /checkout_sessions/{id}/complete
//	POST /checkout_sessions/{id}/cancel
func (s *Service) API() http.Handler { return http.HandlerFunc(s.serve) }

func (s *Service) serve(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	if id := r.Header.Get("Request-Id"); id != "" {
		h.Set("Request-Id", id)
	}
	key := r.Header.Get("Idempotency-Key")
	if r.Method == http.MethodPost && key != "" {
		h.Set("Idempotency-Key", key)
	}

	rt, id, allow := parse(r.URL.Path)
	switch {
	case rt == 0:
		fail(w, fault(http.StatusNotFound, TypeInvalidRequest, "not_found", "No such endpoint.", ""))
		return
	case !slices.Contains(strings.Split(allow, ", "), r.Method):
		notAllowed(w, allow, r.Method)
		return
	}
	if f := version(r); f != nil {
		fail(w, f)
		return
	}
	who, f := s.identify(r)
	if f != nil {
		fail(w, f)
		return
	}
	body, f := readBody(r)
	if f != nil {
		fail(w, f)
		return
	}
	if f := s.signed(r, who, key, body); f != nil {
		fail(w, f)
		return
	}
	if r.Method == http.MethodGet {
		out, err := s.Retrieve(r.Context(), who, id)
		if err != nil {
			fail(w, s.refusal(err, "", ""))
			return
		}
		w.Write(encode(out))
		return
	}
	s.post(w, r, rt, id, who, key, body)
}

// post runs a POST under the idempotency rules of rfc.agentic_checkout.md §6.
func (s *Service) post(w http.ResponseWriter, r *http.Request, rt route, id string, who Caller, key string, body []byte) {
	switch {
	case key == "":
		fail(w, fault(http.StatusBadRequest, TypeInvalidRequest, "idempotency_key_required", "Idempotency-Key header is required on all POST requests", ""))
		return
	case len(key) > maxKey:
		fail(w, fault(http.StatusBadRequest, TypeInvalidRequest, "invalid_idempotency_key", "Idempotency-Key is at most 255 characters.", ""))
		return
	case len(bytes.TrimSpace(body)) == 0 && rt != rCancel:
		fail(w, badJSON("The request body is required."))
		return
	}
	if len(body) > 0 {
		if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
			fail(w, fault(http.StatusUnsupportedMediaType, TypeInvalidRequest, "unsupported_media_type", "Content-Type must be application/json.", ""))
			return
		}
	}
	digest, err := fingerprint(body)
	if err != nil {
		fail(w, badJSON("The request body is not valid JSON."))
		return
	}

	ctx := r.Context()
	scope := who.Org + "\x00" + who.Subject + "\x00" + r.Method + " " + r.URL.Path
	verdict, stored, lease, err := s.cfg.Ledger.Begin(ctx, scope, key, digest)
	if err != nil {
		s.cfg.Report(err)
		fail(w, Unavailable("The idempotency store is unavailable."))
		return
	}
	switch verdict {
	case Replay:
		w.Header().Set("Idempotent-Replayed", "true")
		w.WriteHeader(stored.Status)
		w.Write(stored.Body)
		return
	case Mismatch:
		fail(w, fault(http.StatusUnprocessableEntity, TypeInvalidRequest, "idempotency_conflict", "Idempotency-Key has already been used with a different request body", ""))
		return
	case Busy:
		f := fault(http.StatusConflict, TypeInvalidRequest, "idempotency_in_flight", "A request with this Idempotency-Key is currently being processed", "")
		f.RetryAfter = s.cfg.RetryAfter
		fail(w, f)
		return
	}

	// The key is kept only by a settled answer below 500 (§6.5); a panic or a 5xx releases it.
	kept := false
	defer func() {
		if !kept {
			if err := lease.Release(context.WithoutCancel(ctx)); err != nil {
				s.cfg.Report(err)
			}
		}
	}()
	status, out := s.run(ctx, rt, id, who, key, body)
	res := encode(out)
	if status < http.StatusInternalServerError {
		if err := lease.Settle(context.WithoutCancel(ctx), Stored{Status: status, Body: res}); err != nil {
			s.cfg.Report(err)
		} else {
			kept = true
		}
	}
	w.WriteHeader(status)
	w.Write(res)
}

func (s *Service) run(ctx context.Context, rt route, id string, who Caller, key string, body []byte) (int, any) {
	var (
		out *CheckoutSession
		err error
	)
	status := http.StatusOK
	switch rt {
	case rCreate:
		var req CreateRequest
		if f := decode(body, &req); f != nil {
			return f.Status, f.Body
		}
		status = http.StatusCreated
		out, err = s.Create(ctx, who, &req)
	case rSession:
		var req UpdateRequest
		if f := decode(body, &req); f != nil {
			return f.Status, f.Body
		}
		if f := clearing(body); f != nil {
			return f.Status, f.Body
		}
		out, err = s.Update(ctx, who, id, &req)
	case rComplete:
		var req CompleteRequest
		if f := decode(body, &req); f != nil {
			return f.Status, f.Body
		}
		out, err = s.Complete(ctx, who, id, key, &req)
	case rCancel:
		if len(bytes.TrimSpace(body)) > 0 {
			if f := decode(body, new(CancelRequest)); f != nil {
				return f.Status, f.Body
			}
		}
		out, err = s.Cancel(ctx, who, id)
	}
	if err != nil {
		f := s.refusal(err, "", "")
		return f.Status, f.Body
	}
	return status, out
}

// fail writes a protocol error.
func fail(w http.ResponseWriter, f *Fault) {
	if f.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(f.RetryAfter))
	}
	w.WriteHeader(f.Status)
	w.Write(encode(f.Body))
}

func notAllowed(w http.ResponseWriter, allow, method string) {
	w.Header().Set("Allow", allow)
	fail(w, fault(http.StatusMethodNotAllowed, TypeInvalidRequest, "method_not_allowed", "This endpoint does not take "+method+".", ""))
}

func badJSON(msg string) *Fault {
	return fault(http.StatusBadRequest, TypeInvalidRequest, "invalid_json", msg, "")
}

func encode(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// parse names the endpoint a path is, the session id it carries, and the methods it takes.
func parse(path string) (route, string, string) {
	rest, ok := strings.CutPrefix(path, "/checkout_sessions")
	if !ok {
		return 0, "", ""
	}
	if rest == "" {
		return rCreate, "", http.MethodPost
	}
	parts := strings.Split(strings.TrimPrefix(rest, "/"), "/")
	if !strings.HasPrefix(rest, "/") || parts[0] == "" || len(parts) > 2 {
		return 0, "", ""
	}
	switch {
	case len(parts) == 1:
		return rSession, parts[0], http.MethodGet + ", " + http.MethodPost
	case parts[1] == "complete":
		return rComplete, parts[0], http.MethodPost
	case parts[1] == "cancel":
		return rCancel, parts[0], http.MethodPost
	}
	return 0, "", ""
}

// version enforces API-Version: the server MUST validate it, and a missing or unsupported
// one is answered 400 with the versions it accepts (rfc.agentic_checkout.md §2.1).
func version(r *http.Request) *Fault {
	v := r.Header.Get("API-Version")
	var f *Fault
	switch {
	case v == "":
		f = fault(http.StatusBadRequest, TypeInvalidRequest, "missing_api_version", "The API-Version header is required.", "")
	case v != Version:
		f = fault(http.StatusBadRequest, TypeInvalidRequest, "unsupported_api_version", "API version '"+v+"' is not supported", "")
	default:
		return nil
	}
	f.Body.SupportedVersions = []string{Version}
	return f
}

// identify establishes the caller from the bearer token.
func (s *Service) identify(r *http.Request) (Caller, *Fault) {
	auth := r.Header.Get("Authorization")
	if len(auth) < 8 || !strings.EqualFold(auth[:7], "Bearer ") || strings.TrimSpace(auth[7:]) == "" {
		return Caller{}, unauthorized("Authorization: Bearer <token> is required.")
	}
	who, err := s.cfg.Identify(r)
	if err != nil || who.Org == "" || who.Subject == "" {
		return Caller{}, unauthorized("The bearer token is not valid.")
	}
	return who, nil
}

func unauthorized(msg string) *Fault {
	return fault(http.StatusUnauthorized, TypeInvalidRequest, "unauthorized", msg, "")
}

func invalidSignature(msg string) *Fault {
	return fault(http.StatusUnauthorized, TypeInvalidRequest, "invalid_signature", msg, "")
}

// signed verifies Signature and Timestamp when a Verifier is configured: the timestamp must
// be within the skew of the clock (rfc.agentic_checkout.md §7).
func (s *Service) signed(r *http.Request, who Caller, key string, body []byte) *Fault {
	if s.cfg.Verify == nil {
		return nil
	}
	sig, ts := r.Header.Get("Signature"), r.Header.Get("Timestamp")
	if sig == "" {
		return invalidSignature("Missing Signature header.")
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return invalidSignature("Timestamp must be an RFC 3339 date-time.")
	}
	now := s.cfg.Now()
	if t.Before(now.Add(-s.cfg.Skew)) || t.After(now.Add(s.cfg.Skew)) {
		return invalidSignature("Timestamp outside allowed window.")
	}
	req := Signed{Method: r.Method, Path: requested(r), Key: key, Timestamp: ts, Signature: sig, Body: body}
	if err := s.cfg.Verify.Verify(r.Context(), who, req); err != nil {
		return invalidSignature("Signature verification failed.")
	}
	return nil
}

// requested is the path of the request target as received.
func requested(r *http.Request) string {
	if u, err := url.ParseRequestURI(r.RequestURI); err == nil && u.Path != "" {
		return u.EscapedPath()
	}
	return r.URL.EscapedPath()
}

func readBody(r *http.Request) ([]byte, *Fault) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		var big *http.MaxBytesError
		if errors.As(err, &big) {
			return nil, fault(http.StatusRequestEntityTooLarge, TypeInvalidRequest, "request_too_large", "The request body is too large.", "")
		}
		return nil, badJSON("The request body could not be read.")
	}
	return b, nil
}

// decode reads a body into v, refusing fields the closed request schemas do not define.
func decode(body []byte, v any) *Fault {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var typ *json.UnmarshalTypeError
		if errors.As(err, &typ) {
			return Invalid(path(typ.Field), "The value must be a "+typ.Type.String()+" but is a "+typ.Value+".")
		}
		if msg, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
			return Invalid("", "The field "+msg+" is not defined.")
		}
		return badJSON("The request body is not valid JSON.")
	}
	if _, err := dec.Token(); err != io.EOF {
		return badJSON("The request body is not valid JSON.")
	}
	return nil
}

// path states an encoding/json field path (line_items.0.unit_amount) as an RFC 9535 JSONPath.
func path(field string) string {
	out := "$"
	for _, seg := range strings.Split(field, ".") {
		switch {
		case seg == "":
		case strings.Trim(seg, "0123456789") == "":
			out += "[" + seg + "]"
		default:
			out += "." + seg
		}
	}
	return out
}

// clearing refuses line_items: null, which means clear the field (rfc.agentic_checkout.md §6.2).
func clearing(body []byte) *Fault {
	var raw map[string]json.RawMessage
	if json.Unmarshal(body, &raw) == nil && bytes.Equal(bytes.TrimSpace(raw["line_items"]), []byte("null")) {
		return Invalid("$.line_items", "line_items cannot be cleared.")
	}
	return nil
}

// Discover returns /.well-known/acp.json (rfc.discovery.md) for the host to mount at the origin
// root. base is the API base URL; docs and locales are optional.
func (s *Service) Discover(base, docs string, locales []string) (http.Handler, error) {
	if u, err := url.Parse(base); err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, errors.New("acp: the API base URL must be an absolute https URL")
	}
	doc := encode(Discovery{
		Protocol:   DiscoveryProtocol{Name: "acp", Version: Version, SupportedVersions: []string{Version}, DocumentationURL: docs},
		APIBaseURL: base,
		Transports: []string{"rest"},
		Capabilities: DiscoveryCapabilities{
			Services:            []string{"checkout"},
			SupportedCurrencies: []string{s.cfg.Backend.Currency()},
			SupportedLocales:    locales,
		},
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Type", "application/json")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			notAllowed(w, "GET, HEAD", r.Method)
			return
		}
		// rfc.discovery.md §4.1: at least public, max-age=3600.
		h.Set("Cache-Control", "public, max-age=3600")
		w.Write(doc)
	}), nil
}
