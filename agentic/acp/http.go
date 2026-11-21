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

// Signed is a request as the agent sent it: everything a signature can cover. The spec says
// the client signs the canonical JSON body with an accompanying Timestamp and leaves the
// algorithm, the keys and what else is bound out of band (rfc.agentic_checkout.md §2.1), so
// the Verifier is the host's. A signature over the body alone replays on any endpoint at any
// time; a Verifier binds at least the Timestamp, the Method and the Path as well, and the
// Key on a POST.
type Signed struct {
	Method string
	// Path is the path the agent requested, as received (before the host strips a prefix).
	Path string
	// Key is the Idempotency-Key of a POST, empty on a GET.
	Key string
	// Timestamp is the Timestamp header exactly as sent; the service has already required it
	// to be an RFC 3339 date-time within the clock skew.
	Timestamp string
	// Signature is the Signature header exactly as sent.
	Signature string
	// Body is the raw request body.
	Body []byte
}

// Verifier checks the Signature an agent sent over a request.
type Verifier interface {
	Verify(ctx context.Context, who Caller, req Signed) error
}

// Request limits and hints the spec states or leaves to the server.
const (
	// maxBody bounds a request body: a checkout request is a few kilobytes.
	maxBody = 1 << 20
	// maxKey is the longest Idempotency-Key (rfc.agentic_checkout.md §6.1).
	maxKey = 255
	// retryAfter is the Retry-After, in seconds, of an idempotency_in_flight answer.
	retryAfter = 1
)

type route int

const (
	rCreate route = iota + 1
	rSession
	rComplete
	rCancel
)

// API returns the Agentic Checkout REST API as an http.Handler. It answers the paths of
// openapi.agentic_checkout.yaml relative to the API base URL: the host mounts it under that
// URL's path and strips the prefix.
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
	h.Set("Cache-Control", "no-store")
	if id := r.Header.Get("Request-Id"); id != "" && len(id) <= maxKey {
		h.Set("Request-Id", id)
	}
	key := r.Header.Get("Idempotency-Key")
	if r.Method == http.MethodPost && key != "" && len(key) <= maxKey {
		h.Set("Idempotency-Key", key)
	}

	rt, id, allow := parse(r.URL.Path)
	switch {
	case rt == 0:
		s.fail(w, fault(http.StatusNotFound, TypeInvalidRequest, "not_found", "No such endpoint.", ""))
		return
	case !slices.Contains(strings.Split(allow, ", "), r.Method):
		h.Set("Allow", allow)
		s.fail(w, fault(http.StatusMethodNotAllowed, TypeInvalidRequest, "method_not_allowed", "This endpoint does not take "+r.Method+".", ""))
		return
	}

	// API-Version, then who is asking (rfc.agentic_checkout.md §2.1, §3.1).
	if f := version(r); f != nil {
		s.fail(w, f)
		return
	}
	who, f := s.identify(r)
	if f != nil {
		s.fail(w, f)
		return
	}
	body, f := readBody(w, r)
	if f != nil {
		s.fail(w, f)
		return
	}
	if f := s.signed(r, who, key, body); f != nil {
		s.fail(w, f)
		return
	}

	if r.Method == http.MethodGet {
		out, err := s.Retrieve(r.Context(), who, id)
		s.answer(w, http.StatusOK, out, err)
		return
	}
	s.post(w, r, rt, id, who, key, body)
}

// post runs a POST under the idempotency rules of rfc.agentic_checkout.md §6.
func (s *Service) post(w http.ResponseWriter, r *http.Request, rt route, id string, who Caller, key string, body []byte) {
	switch {
	case key == "":
		s.fail(w, fault(http.StatusBadRequest, TypeInvalidRequest, "idempotency_key_required", "Idempotency-Key header is required on all POST requests", ""))
		return
	case len(key) > maxKey:
		s.fail(w, fault(http.StatusBadRequest, TypeInvalidRequest, "invalid_idempotency_key", "Idempotency-Key is at most 255 characters.", ""))
		return
	}
	if len(bytes.TrimSpace(body)) == 0 && rt != rCancel {
		s.fail(w, fault(http.StatusBadRequest, TypeInvalidRequest, "invalid_json", "The request body is required.", ""))
		return
	}
	if len(body) > 0 {
		if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
			s.fail(w, fault(http.StatusUnsupportedMediaType, TypeInvalidRequest, "unsupported_media_type", "Content-Type must be application/json.", ""))
			return
		}
	}
	digest, err := Fingerprint(body)
	if err != nil {
		s.fail(w, fault(http.StatusBadRequest, TypeInvalidRequest, "invalid_json", "The request body is not valid JSON.", ""))
		return
	}

	ctx := r.Context()
	scope := who.Org + "\x00" + who.Subject + "\x00" + r.Method + " " + r.URL.Path
	verdict, stored, lease, err := s.cfg.Ledger.Begin(ctx, scope, key, digest)
	if err != nil {
		s.cfg.Report(err)
		s.fail(w, Unavailable("The idempotency store is unavailable."))
		return
	}
	switch verdict {
	case Replay:
		w.Header().Set("Idempotent-Replayed", "true")
		w.WriteHeader(stored.Status)
		w.Write(stored.Body)
		return
	case Mismatch:
		s.fail(w, fault(http.StatusUnprocessableEntity, TypeInvalidRequest, "idempotency_conflict", "Idempotency-Key has already been used with a different request body", ""))
		return
	case Busy:
		f := fault(http.StatusConflict, TypeInvalidRequest, "idempotency_in_flight", "A request with this Idempotency-Key is currently being processed", "")
		f.RetryAfter = retryAfter
		s.fail(w, f)
		return
	}

	// The key is held until the request settles or releases it. A panic below leaves neither,
	// so it releases the key on the way out: the agent's retry runs instead of waiting out a
	// claim nobody holds.
	settled := false
	defer func() {
		if !settled {
			if err := lease.Release(context.WithoutCancel(ctx)); err != nil {
				s.cfg.Report(err)
			}
		}
	}()
	status, out := s.run(ctx, rt, id, who, key, body)
	res, err := json.Marshal(out)
	if err != nil {
		s.cfg.Report(err)
		status, res = http.StatusInternalServerError, mustJSON(Internal().Body)
	}
	settled = true
	// Whatever the ledger does from here, the request has run: the answer goes out.
	if ctx := context.WithoutCancel(ctx); status >= 500 {
		if err := lease.Release(ctx); err != nil {
			s.cfg.Report(err)
		}
	} else if err := lease.Settle(ctx, Stored{Status: status, Body: res}); err != nil {
		s.cfg.Report(err)
	}
	w.WriteHeader(status)
	w.Write(res)
}

// run decodes and executes one POST, answering the status and the body.
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
		var req CancelRequest
		if len(bytes.TrimSpace(body)) > 0 {
			if f := decode(body, &req); f != nil {
				return f.Status, f.Body
			}
		}
		out, err = s.Cancel(ctx, who, id, &req)
	}
	if err != nil {
		f := s.refusal(err, "", "")
		return f.Status, f.Body
	}
	return status, out
}

// answer writes the outcome of a request that is not kept against an idempotency key.
func (s *Service) answer(w http.ResponseWriter, status int, out *CheckoutSession, err error) {
	if err != nil {
		s.fail(w, s.refusal(err, "", ""))
		return
	}
	w.WriteHeader(status)
	w.Write(mustJSON(out))
}

// fail writes a protocol error.
func (s *Service) fail(w http.ResponseWriter, f *Fault) {
	if f.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(f.RetryAfter))
	}
	w.WriteHeader(f.Status)
	w.Write(mustJSON(f.Body))
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		// Every value written here is a plain struct of strings, integers and slices.
		return []byte(`{"type":"processing_error","code":"internal_error","message":"The request could not be processed."}`)
	}
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
	if !strings.HasPrefix(rest, "/") || parts[0] == "" || len(parts[0]) > maxKey || len(parts) > 2 {
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

// signed verifies the Signature and Timestamp of a request when the host configured a
// Verifier: both are required then, and the timestamp must be within the skew
// (rfc.agentic_checkout.md §7).
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
	// Compared as instants, never as a Duration: a timestamp centuries away overflows one.
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

// requested is the path the agent asked for: the request target as received, which a host's
// prefix stripping does not change.
func requested(r *http.Request) string {
	if u, err := url.ParseRequestURI(r.RequestURI); err == nil && u.Path != "" {
		return u.EscapedPath()
	}
	return r.URL.EscapedPath()
}

// readBody reads the request body under the size bound.
func readBody(w http.ResponseWriter, r *http.Request) ([]byte, *Fault) {
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		var big *http.MaxBytesError
		if errors.As(err, &big) {
			return nil, fault(http.StatusRequestEntityTooLarge, TypeInvalidRequest, "request_too_large", "The request body is too large.", "")
		}
		return nil, fault(http.StatusBadRequest, TypeInvalidRequest, "invalid_json", "The request body could not be read.", "")
	}
	return b, nil
}

// decode reads a body into v, refusing a field the schema does not define (every request
// schema is closed with additionalProperties false) and data after the document.
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
		return fault(http.StatusBadRequest, TypeInvalidRequest, "invalid_json", "The request body is not valid JSON.", "")
	}
	if _, err := dec.Token(); err != io.EOF {
		return fault(http.StatusBadRequest, TypeInvalidRequest, "invalid_json", "The request body is not valid JSON.", "")
	}
	return nil
}

// path states a field path of encoding/json (line_items.0.unit_amount) as an RFC 9535
// JSONPath ($.line_items[0].unit_amount).
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

// clearing refuses an update that sets line_items to null, which means clear the field
// (rfc.agentic_checkout.md §6.2): a session cannot be left without an item.
func clearing(body []byte) *Fault {
	var raw map[string]json.RawMessage
	if json.Unmarshal(body, &raw) == nil && bytes.Equal(bytes.TrimSpace(raw["line_items"]), []byte("null")) {
		return Invalid("$.line_items", "line_items cannot be cleared.")
	}
	return nil
}

// Discover returns the discovery document served at /.well-known/acp.json (rfc.discovery.md)
// as an http.Handler: the host mounts it at that path on the origin root. base is the API base URL
// the agent appends the checkout paths to; docs and locales are optional.
func (s *Service) Discover(base, docs string, locales []string) (http.Handler, error) {
	if u, err := url.Parse(base); err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, errors.New("acp: the API base URL must be an absolute https URL")
	}
	doc := mustJSON(Discovery{
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
			h.Set("Allow", "GET, HEAD")
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write(mustJSON(fault(http.StatusMethodNotAllowed, TypeInvalidRequest, "method_not_allowed", "This endpoint does not take "+r.Method+".", "").Body))
			return
		}
		// A minimum of one hour (rfc.discovery.md §4.1).
		h.Set("Cache-Control", "public, max-age=3600")
		w.Write(doc)
	}), nil
}
