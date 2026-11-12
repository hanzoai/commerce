// Package paypaltest serves PayPal's REST contract over HTTP for tests that
// drive the real paypal.Provider through its HTTP boundary.
//
// It is not a fake written to agree with the code under test. Every refusal it
// makes is one PayPal makes: the limits are the Orders v2 OpenAPI spec's, and
// the error bodies are PayPal's own, recorded from api-m.paypal.com. Every
// success body is PayPal's published example for that endpoint, with the
// order's own id, status and amount put in. contract.json says where each came
// from.
//
// What the contract cannot say is what a buyer does in PayPal's window, so the
// one thing a test does by hand is Approve an order — the step a real buyer
// takes on paypal.com between create and capture.
package paypaltest

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

//go:embed contract.json
var contractJSON []byte

type contract struct {
	Limits   map[string]int `json:"limits"`
	Recorded struct {
		OAuthInvalidClient    response `json:"oauth_invalid_client"`
		OrdersMissingRequired response `json:"orders_missing_required"`
		RequestIDTooLong      response `json:"request_id_too_long"`
		OAuthTokenScope       string   `json:"oauth_token_scope"`
	} `json:"recorded"`
	Examples map[string]json.RawMessage `json:"examples"`
}

type response struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

// Order is one order as the server holds it.
type Order struct {
	ID        string
	Intent    string
	Status    string // CREATED, APPROVED, COMPLETED
	Currency  string
	Value     string
	CustomID  string
	RequestID string // the PayPal-Request-Id it was created under
}

// Server is PayPal's REST API for one merchant app.
type Server struct {
	*httptest.Server
	ClientID, ClientSecret string

	c          contract
	mu         sync.Mutex
	seq        int
	orders     map[string]*Order
	replies    map[string][]byte // PayPal-Request-Id + path -> the first answer
	RequestIDs []string          // every PayPal-Request-Id received, in order
}

// New starts a server for the app identified by clientID and clientSecret.
func New(t testing.TB, clientID, clientSecret string) *Server {
	t.Helper()
	var c contract
	if err := json.Unmarshal(contractJSON, &c); err != nil {
		t.Fatalf("paypaltest: contract.json: %v", err)
	}
	s := &Server{ClientID: clientID, ClientSecret: clientSecret, c: c,
		orders: map[string]*Order{}, replies: map[string][]byte{}}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.Close)
	return s
}

// Approve is the buyer approving the order on paypal.com.
func (s *Server) Approve(orderID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o := s.orders[orderID]; o != nil && o.Status == "CREATED" {
		o.Status = "APPROVED"
	}
}

// Order returns the order as the server holds it, or nil.
func (s *Server) Order(id string) *Order {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o := s.orders[id]; o != nil {
		cp := *o
		return &cp
	}
	return nil
}

// Limit is the contract's maximum length for a field or header.
func (s *Server) Limit(name string) int { return s.c.Limits[name] }

const token = "A21AAcontract-token"

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/v1/oauth2/token" {
		s.oauth(w, r)
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+token {
		write(w, http.StatusUnauthorized, json.RawMessage(`{"error":"invalid_token","error_description":"Token signature verification failed"}`))
		return
	}
	id := r.Header.Get("PayPal-Request-Id")
	if r.Method == http.MethodPost {
		s.mu.Lock()
		s.RequestIDs = append(s.RequestIDs, id)
		s.mu.Unlock()
		if len(id) > s.c.Limits["PayPal-Request-Id"] {
			body := strings.Replace(string(s.c.Recorded.RequestIDTooLong.Body), `"value":""`, `"value":`+quote(id), 1)
			write(w, s.c.Recorded.RequestIDTooLong.Status, json.RawMessage(body))
			return
		}
		// PayPal answers a repeated PayPal-Request-Id with the first answer.
		if id != "" {
			s.mu.Lock()
			prior, ok := s.replies[id+" "+r.URL.Path]
			s.mu.Unlock()
			if ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(prior)
				return
			}
		}
	}
	rec := httptest.NewRecorder()
	s.route(rec, r, id)
	if r.Method == http.MethodPost && id != "" && rec.Code < 300 {
		s.mu.Lock()
		s.replies[id+" "+r.URL.Path] = rec.Body.Bytes()
		s.mu.Unlock()
	}
	for k, v := range rec.Header() {
		w.Header()[k] = v
	}
	w.WriteHeader(rec.Code)
	_, _ = w.Write(rec.Body.Bytes())
}

func (s *Server) oauth(w http.ResponseWriter, r *http.Request) {
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(s.ClientID+":"+s.ClientSecret))
	if r.Header.Get("Authorization") != want {
		write(w, s.c.Recorded.OAuthInvalidClient.Status, s.c.Recorded.OAuthInvalidClient.Body)
		return
	}
	write(w, http.StatusOK, map[string]any{
		"scope": s.c.Recorded.OAuthTokenScope, "access_token": token, "token_type": "Bearer",
		"app_id": "APP-CONTRACT", "expires_in": 32400,
	})
}

func (s *Server) route(w http.ResponseWriter, r *http.Request, requestID string) {
	path := r.URL.Path
	switch {
	case r.Method == http.MethodPost && path == "/v2/checkout/orders":
		s.create(w, r, requestID)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/v2/checkout/orders/") && strings.HasSuffix(path, "/capture"):
		s.settle(w, strings.TrimSuffix(strings.TrimPrefix(path, "/v2/checkout/orders/"), "/capture"), "CAPTURE")
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/v2/checkout/orders/") && strings.HasSuffix(path, "/authorize"):
		s.settle(w, strings.TrimSuffix(strings.TrimPrefix(path, "/v2/checkout/orders/"), "/authorize"), "AUTHORIZE")
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/v2/payments/authorizations/") && strings.HasSuffix(path, "/capture"):
		s.captureAuthorization(w, strings.TrimSuffix(strings.TrimPrefix(path, "/v2/payments/authorizations/"), "/capture"))
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/v2/payments/authorizations/") && strings.HasSuffix(path, "/void"):
		w.WriteHeader(http.StatusNoContent)
	default:
		write(w, http.StatusNotFound, json.RawMessage(`{"name":"RESOURCE_NOT_FOUND","message":"The specified resource does not exist."}`))
	}
}

func (s *Server) create(w http.ResponseWriter, r *http.Request, requestID string) {
	var in struct {
		Intent        string `json:"intent"`
		PurchaseUnits []struct {
			Amount struct {
				CurrencyCode string `json:"currency_code"`
				Value        string `json:"value"`
			} `json:"amount"`
			CustomID    string `json:"custom_id"`
			InvoiceID   string `json:"invoice_id"`
			ReferenceID string `json:"reference_id"`
			Description string `json:"description"`
		} `json:"purchase_units"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Intent == "" || len(in.PurchaseUnits) == 0 {
		write(w, s.c.Recorded.OrdersMissingRequired.Status, s.c.Recorded.OrdersMissingRequired.Body)
		return
	}
	if in.Intent != "CAPTURE" && in.Intent != "AUTHORIZE" {
		write(w, http.StatusBadRequest, invalid("/intent", "INVALID_PARAMETER_VALUE"))
		return
	}
	pu := in.PurchaseUnits[0]
	for field, v := range map[string]string{"custom_id": pu.CustomID, "invoice_id": pu.InvoiceID, "reference_id": pu.ReferenceID, "description": pu.Description} {
		if len(v) > s.c.Limits[field] {
			write(w, http.StatusBadRequest, invalid("/purchase_units/@reference_id=='default'/"+field, "INVALID_STRING_LENGTH"))
			return
		}
	}
	if len(pu.Amount.CurrencyCode) != 3 || pu.Amount.Value == "" {
		write(w, http.StatusBadRequest, invalid("/purchase_units/@reference_id=='default'/amount", "MISSING_REQUIRED_PARAMETER"))
		return
	}
	s.mu.Lock()
	s.seq++
	o := &Order{ID: fmt.Sprintf("5O190127TN36%05d", s.seq), Intent: in.Intent, Status: "CREATED",
		Currency: pu.Amount.CurrencyCode, Value: pu.Amount.Value, CustomID: pu.CustomID, RequestID: requestID}
	s.orders[o.ID] = o
	s.mu.Unlock()

	var body map[string]any
	_ = json.Unmarshal(s.c.Examples["orders_create"], &body)
	body["id"] = o.ID
	body["status"] = o.Status
	body["intent"] = o.Intent
	write(w, http.StatusCreated, body)
}

func (s *Server) settle(w http.ResponseWriter, orderID, intent string) {
	s.mu.Lock()
	o := s.orders[orderID]
	approved := o != nil && o.Status == "APPROVED" && o.Intent == intent
	if approved {
		o.Status = "COMPLETED"
	}
	s.mu.Unlock()
	if o == nil {
		write(w, http.StatusNotFound, json.RawMessage(`{"name":"RESOURCE_NOT_FOUND","details":[{"issue":"INVALID_RESOURCE_ID"}],"message":"The specified resource does not exist."}`))
		return
	}
	if !approved {
		write(w, http.StatusUnprocessableEntity, s.c.Examples["order_not_approved"])
		return
	}
	name, key := "orders_capture", "captures"
	if intent == "AUTHORIZE" {
		name, key = "orders_authorize", "authorizations"
	}
	var body map[string]any
	_ = json.Unmarshal(s.c.Examples[name], &body)
	body["id"] = o.ID
	body["status"] = "COMPLETED"
	units, _ := body["purchase_units"].([]any)
	if len(units) > 0 {
		pu, _ := units[0].(map[string]any)
		pays, _ := pu["payments"].(map[string]any)
		list, _ := pays[key].([]any)
		if len(list) > 0 {
			first, _ := list[0].(map[string]any)
			first["id"] = o.ID + "-" + strings.ToLower(intent)
			first["amount"] = map[string]string{"currency_code": o.Currency, "value": o.Value}
		}
	}
	write(w, http.StatusCreated, body)
}

func (s *Server) captureAuthorization(w http.ResponseWriter, authID string) {
	orderID := strings.TrimSuffix(authID, "-authorize")
	s.mu.Lock()
	o := s.orders[orderID]
	s.mu.Unlock()
	if o == nil || o.Intent != "AUTHORIZE" || o.Status != "COMPLETED" {
		write(w, http.StatusNotFound, json.RawMessage(`{"name":"RESOURCE_NOT_FOUND","details":[{"issue":"INVALID_RESOURCE_ID"}],"message":"The specified resource does not exist."}`))
		return
	}
	var body map[string]any
	_ = json.Unmarshal(s.c.Examples["authorizations_capture"], &body)
	body["id"] = orderID + "-capture"
	body["status"] = "COMPLETED"
	body["amount"] = map[string]string{"currency_code": o.Currency, "value": o.Value}
	write(w, http.StatusCreated, body)
}

func invalid(field, issue string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"name":"INVALID_REQUEST","message":"Request is not well-formed, syntactically incorrect, or violates schema.","details":[{"field":%s,"location":"body","issue":%s}]}`, quote(field), quote(issue)))
}

func quote(s string) string { b, _ := json.Marshal(s); return string(b) }

func write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	switch b := body.(type) {
	case json.RawMessage:
		_, _ = w.Write(b)
	default:
		_ = json.NewEncoder(w).Encode(b)
	}
}

// Transport routes requests for PayPal's API hosts to the server, so a provider
// built with its production base URL reaches the contract unchanged.
func (s *Server) Transport(base http.RoundTripper) http.RoundTripper {
	return roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "api-m.paypal.com" || r.URL.Host == "api-m.sandbox.paypal.com" {
			r = r.Clone(r.Context())
			r.URL.Scheme = "http"
			r.URL.Host = strings.TrimPrefix(s.URL, "http://")
		}
		return base.RoundTrip(r)
	})
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Route sends every request for PayPal's API hosts through http.DefaultTransport
// to this server for the rest of the test.
func (s *Server) Route(t testing.TB) {
	t.Helper()
	orig := http.DefaultTransport
	http.DefaultTransport = s.Transport(orig)
	t.Cleanup(func() { http.DefaultTransport = orig })
}
