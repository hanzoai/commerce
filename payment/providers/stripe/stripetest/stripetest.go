// Package stripetest serves Stripe's PaymentIntents and Refunds contract over
// HTTP for tests that drive the real stripe.Provider through its HTTP boundary.
//
// The contract is Stripe's, not ours: which parameters each endpoint accepts
// and requires comes from Stripe's OpenAPI spec, the objects it answers with
// are Stripe's published fixtures (the ones stripe-mock serves), and what a
// test payment method does, how an Idempotency-Key replays and what an error
// says follow Stripe's documented test mode. contract.json names each source.
// Stripe's own test mode needs a test secret key, which this environment does
// not hold; this is the next most faithful thing.
package stripetest

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

//go:embed contract.json
var contractJSON []byte

type endpoint struct {
	Params   []string `json:"params"`
	Required []string `json:"required"`
}

type contract struct {
	Endpoints          map[string]endpoint        `json:"endpoints"`
	IdempotencyKeyMax  int                        `json:"idempotency_key_max"`
	Fixtures           map[string]json.RawMessage `json:"fixtures"`
	TestPaymentMethods map[string]string          `json:"test_payment_methods"`
}

// Intent is one PaymentIntent as the server holds it.
type Intent struct {
	ID            string
	Amount        int64
	Currency      string
	Status        string
	CaptureMethod string
	PaymentMethod string
	Params        url.Values // the form it was created with
}

// Server is Stripe's API for one account.
type Server struct {
	*httptest.Server
	SecretKey string

	c       contract
	mu      sync.Mutex
	seq     int
	intents map[string]*Intent
	replies map[string]reply // Idempotency-Key -> the first answer
	Keys    []string         // every Idempotency-Key received, in order
}

type reply struct {
	form   string
	status int
	body   []byte
}

// New starts a server for the account whose secret key is secretKey.
func New(t testing.TB, secretKey string) *Server {
	t.Helper()
	var c contract
	if err := json.Unmarshal(contractJSON, &c); err != nil {
		t.Fatalf("stripetest: contract.json: %v", err)
	}
	s := &Server{SecretKey: secretKey, c: c, intents: map[string]*Intent{}, replies: map[string]reply{}}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.Close)
	return s
}

// Route sends every request for api.stripe.com through http.DefaultTransport to
// this server for the rest of the test.
func (s *Server) Route(t testing.TB) {
	t.Helper()
	orig := http.DefaultTransport
	http.DefaultTransport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "api.stripe.com" {
			r = r.Clone(r.Context())
			r.URL.Scheme = "http"
			r.URL.Host = strings.TrimPrefix(s.URL, "http://")
		}
		return orig.RoundTrip(r)
	})
	t.Cleanup(func() { http.DefaultTransport = orig })
}

// Intent returns the PaymentIntent as the server holds it, or nil.
func (s *Server) Intent(id string) *Intent {
	s.mu.Lock()
	defer s.mu.Unlock()
	if in := s.intents[id]; in != nil {
		cp := *in
		return &cp
	}
	return nil
}

// Intents is how many PaymentIntents the account holds.
func (s *Server) Intents() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.intents)
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Basic "+base64.StdEncoding.EncodeToString([]byte(s.SecretKey+":")) {
		fail(w, http.StatusUnauthorized, "invalid_request_error", "", "", "Invalid API Key provided.")
		return
	}
	if err := r.ParseForm(); err != nil {
		fail(w, http.StatusBadRequest, "invalid_request_error", "", "", "Invalid request body.")
		return
	}
	form := r.PostForm
	key := r.Header.Get("Idempotency-Key")
	if r.Method == http.MethodPost && key != "" {
		s.mu.Lock()
		s.Keys = append(s.Keys, key)
		prior, seen := s.replies[key]
		s.mu.Unlock()
		if len(key) > s.c.IdempotencyKeyMax {
			fail(w, http.StatusBadRequest, "invalid_request_error", "", "",
				fmt.Sprintf("Invalid Idempotency-Key: the key is longer than %d characters.", s.c.IdempotencyKeyMax))
			return
		}
		if seen {
			if prior.form != r.URL.Path+"?"+form.Encode() {
				fail(w, http.StatusBadRequest, "idempotency_error", "", "",
					"Keys for idempotent requests can only be used with the same parameters they were first used with.")
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Idempotent-Replayed", "true")
			w.WriteHeader(prior.status)
			_, _ = w.Write(prior.body)
			return
		}
	}
	rec := httptest.NewRecorder()
	s.route(rec, r, form)
	if r.Method == http.MethodPost && key != "" && rec.Code != http.StatusUnauthorized {
		s.mu.Lock()
		s.replies[key] = reply{form: r.URL.Path + "?" + form.Encode(), status: rec.Code, body: rec.Body.Bytes()}
		s.mu.Unlock()
	}
	for k, v := range rec.Header() {
		w.Header()[k] = v
	}
	w.WriteHeader(rec.Code)
	_, _ = w.Write(rec.Body.Bytes())
}

func (s *Server) route(w http.ResponseWriter, r *http.Request, form url.Values) {
	path := strings.TrimPrefix(r.URL.Path, "/v1")
	switch {
	case r.Method == http.MethodPost && path == "/payment_intents":
		if s.check(w, "/v1/payment_intents", form) {
			s.create(w, form)
		}
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/payment_intents/") && strings.HasSuffix(path, "/capture"):
		if s.check(w, "/v1/payment_intents/{intent}/capture", form) {
			s.capture(w, strings.TrimSuffix(strings.TrimPrefix(path, "/payment_intents/"), "/capture"), form)
		}
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/payment_intents/") && strings.HasSuffix(path, "/cancel"):
		if s.check(w, "/v1/payment_intents/{intent}/cancel", form) {
			s.cancel(w, strings.TrimSuffix(strings.TrimPrefix(path, "/payment_intents/"), "/cancel"))
		}
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/payment_intents/"):
		s.mu.Lock()
		in := s.intents[strings.TrimPrefix(path, "/payment_intents/")]
		s.mu.Unlock()
		if in == nil {
			fail(w, http.StatusNotFound, "invalid_request_error", "resource_missing", "intent", "No such payment_intent")
			return
		}
		s.writeIntent(w, http.StatusOK, in)
	case r.Method == http.MethodPost && path == "/refunds":
		if s.check(w, "/v1/refunds", form) {
			s.refund(w, form)
		}
	default:
		fail(w, http.StatusNotFound, "invalid_request_error", "", "", "Unrecognized request URL ("+r.Method+": "+r.URL.Path+").")
	}
}

// check holds a form to the endpoint's spec: every parameter is one it
// declares, every required one is present.
func (s *Server) check(w http.ResponseWriter, ep string, form url.Values) bool {
	spec := s.c.Endpoints[ep]
	known := map[string]bool{}
	for _, p := range spec.Params {
		known[p] = true
	}
	names := make([]string, 0, len(form))
	for k := range form {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		top := k
		if i := strings.IndexByte(k, '['); i >= 0 {
			top = k[:i]
		}
		if !known[top] {
			fail(w, http.StatusBadRequest, "invalid_request_error", "parameter_unknown", k, "Received unknown parameter: "+k)
			return false
		}
	}
	for _, p := range spec.Required {
		if form.Get(p) == "" {
			fail(w, http.StatusBadRequest, "invalid_request_error", "parameter_missing", p, "Missing required param: "+p+".")
			return false
		}
	}
	return true
}

func (s *Server) create(w http.ResponseWriter, form url.Values) {
	amount, err := strconv.ParseInt(form.Get("amount"), 10, 64)
	if err != nil || amount <= 0 {
		fail(w, http.StatusBadRequest, "invalid_request_error", "parameter_invalid_integer", "amount", "Invalid integer: "+form.Get("amount"))
		return
	}
	s.mu.Lock()
	s.seq++
	in := &Intent{
		ID:            fmt.Sprintf("pi_contract%06d", s.seq),
		Amount:        amount,
		Currency:      form.Get("currency"),
		Status:        "requires_payment_method",
		CaptureMethod: "automatic",
		PaymentMethod: form.Get("payment_method"),
		Params:        form,
	}
	if form.Get("capture_method") == "manual" {
		in.CaptureMethod = "manual"
	}
	s.intents[in.ID] = in
	s.mu.Unlock()

	if form.Get("confirm") != "true" || in.PaymentMethod == "" {
		s.writeIntent(w, http.StatusOK, in)
		return
	}
	outcome, ok := s.c.TestPaymentMethods[in.PaymentMethod]
	if !ok {
		fail(w, http.StatusBadRequest, "invalid_request_error", "resource_missing", "payment_method", "No such PaymentMethod: '"+in.PaymentMethod+"'")
		return
	}
	s.mu.Lock()
	switch outcome {
	case "succeeded":
		in.Status = "succeeded"
		if in.CaptureMethod == "manual" {
			in.Status = "requires_capture"
		}
	case "requires_action":
		in.Status = "requires_action"
	case "card_declined":
		in.Status = "requires_payment_method"
	}
	s.mu.Unlock()
	if outcome == "card_declined" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
			"type": "card_error", "code": "card_declined", "decline_code": "generic_decline",
			"message": "Your card was declined.", "payment_intent": s.object(in),
		}})
		return
	}
	s.writeIntent(w, http.StatusOK, in)
}

func (s *Server) capture(w http.ResponseWriter, id string, form url.Values) {
	s.mu.Lock()
	in := s.intents[id]
	if in != nil && in.Status == "requires_capture" {
		if v := form.Get("amount_to_capture"); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				in.Amount = n
			}
		}
		in.Status = "succeeded"
	}
	s.mu.Unlock()
	switch {
	case in == nil:
		fail(w, http.StatusNotFound, "invalid_request_error", "resource_missing", "intent", "No such payment_intent: '"+id+"'")
	case in.Status != "succeeded":
		fail(w, http.StatusBadRequest, "invalid_request_error", "payment_intent_unexpected_state", "",
			"This PaymentIntent could not be captured because it has a status of "+in.Status+".")
	default:
		s.writeIntent(w, http.StatusOK, in)
	}
}

func (s *Server) cancel(w http.ResponseWriter, id string) {
	s.mu.Lock()
	in := s.intents[id]
	ok := in != nil && in.Status != "succeeded" && in.Status != "canceled"
	if ok {
		in.Status = "canceled"
	}
	s.mu.Unlock()
	if !ok {
		fail(w, http.StatusBadRequest, "invalid_request_error", "payment_intent_unexpected_state", "", "This PaymentIntent cannot be canceled.")
		return
	}
	s.writeIntent(w, http.StatusOK, in)
}

func (s *Server) refund(w http.ResponseWriter, form url.Values) {
	s.mu.Lock()
	in := s.intents[form.Get("payment_intent")]
	s.mu.Unlock()
	if in == nil || in.Status != "succeeded" {
		fail(w, http.StatusBadRequest, "invalid_request_error", "charge_not_refundable", "payment_intent", "This PaymentIntent does not have a successful charge to refund.")
		return
	}
	var body map[string]any
	_ = json.Unmarshal(s.c.Fixtures["refund"], &body)
	amount := in.Amount
	if v := form.Get("amount"); v != "" {
		amount, _ = strconv.ParseInt(v, 10, 64)
	}
	body["id"] = "re_" + strings.TrimPrefix(in.ID, "pi_")
	body["amount"] = amount
	body["currency"] = in.Currency
	body["payment_intent"] = in.ID
	body["status"] = "succeeded"
	write(w, http.StatusOK, body)
}

func (s *Server) object(in *Intent) map[string]any {
	var body map[string]any
	_ = json.Unmarshal(s.c.Fixtures["payment_intent"], &body)
	body["id"] = in.ID
	body["amount"] = in.Amount
	body["currency"] = in.Currency
	body["status"] = in.Status
	body["capture_method"] = in.CaptureMethod
	body["last_payment_error"] = nil
	body["latest_charge"] = nil
	body["client_secret"] = in.ID + "_secret_contract"
	if in.Status == "succeeded" {
		body["latest_charge"] = "ch_" + strings.TrimPrefix(in.ID, "pi_")
		body["amount_received"] = in.Amount
	}
	if in.PaymentMethod != "" {
		body["payment_method"] = in.PaymentMethod
	}
	return body
}

func (s *Server) writeIntent(w http.ResponseWriter, status int, in *Intent) {
	s.mu.Lock()
	body := s.object(in)
	s.mu.Unlock()
	write(w, status, body)
}

func fail(w http.ResponseWriter, status int, typ, code, param, msg string) {
	e := map[string]any{"type": typ, "message": msg}
	if code != "" {
		e["code"] = code
	}
	if param != "" {
		e["param"] = param
	}
	write(w, status, map[string]any{"error": e})
}

func write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
