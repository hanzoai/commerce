// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package circle

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// server answers each request with handle, after recording it.
type server struct {
	*httptest.Server
	method, path, query string
	body                map[string]any
	raw                 []byte
}

func serve(t *testing.T, handle func(w http.ResponseWriter, r *http.Request)) *server {
	t.Helper()
	s := &server{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.method, s.path, s.query = r.Method, r.URL.Path, r.URL.RawQuery
		s.raw, _ = io.ReadAll(r.Body)
		s.body = nil
		_ = json.Unmarshal(s.raw, &s.body)
		if r.Header.Get("Authorization") != "" {
			t.Errorf("a permissionless Gateway call carried an Authorization header")
		}
		handle(w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

func answer(status int, body string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

func TestBaseIsChosenByNetwork(t *testing.T) {
	if Mainnet.Base() != "https://gateway-api.circle.com" || Testnet.Base() != "https://gateway-api-testnet.circle.com" {
		t.Fatal(Mainnet.Base(), Testnet.Base())
	}
}

func TestInfoAndBalancesAndDeposits(t *testing.T) {
	ctx := context.Background()
	s := serve(t, answer(200, `{"version":1,"domains":[{"chain":"Base","network":"Mainnet","domain":6,"walletContract":{"address":"0x77777777Dcc4d5A8B6E418Fd04D8997ef11000eE","supportedTokens":["USDC"]},"processedHeight":"1","burnIntentExpirationHeight":"2"}]}`))
	info, err := New(s.URL).Info(ctx)
	if err != nil || s.method != "GET" || s.path != "/v1/info" || len(info.Domains) != 1 || info.Domains[0].Domain != 6 || info.Domains[0].Wallet.Tokens[0] != "USDC" {
		t.Fatalf("info: %+v %v (%s %s)", info, err, s.method, s.path)
	}

	s = serve(t, answer(200, `{"token":"USDC","balances":[{"domain":6,"depositor":"0xabc","balance":"12.500000"}]}`))
	bals, err := New(s.URL).Balances(ctx, USDC, "0xabc")
	if err != nil || s.method != "POST" || s.path != "/v1/balances" || len(bals) != 1 || bals[0].Balance != "12.500000" {
		t.Fatalf("balances: %+v %v", bals, err)
	}
	if string(s.raw) != `{"token":"USDC","sources":[{"depositor":"0xabc"}]}` {
		t.Fatalf("balances body = %s — with no domain Gateway answers for every domain", s.raw)
	}

	s = serve(t, answer(200, `{"token":"USDC","deposits":[{"depositor":"0xabc","domain":0,"transactionHash":"0x01","amount":"2500000","status":"pending","blockHeight":"9","blockHash":"0x02","blockTimestamp":"2026-01-01T00:00:00Z"}]}`))
	deps, err := New(s.URL).Deposits(ctx, USDC, "0xabc")
	if err != nil || s.path != "/v1/deposits" || len(deps) != 1 || deps[0].Amount != "2500000" {
		t.Fatalf("deposits: %+v %v", deps, err)
	}
}

func TestEstimateSendsTheIntentsAsAnArray(t *testing.T) {
	s := serve(t, answer(200, `{"body":[{"burnIntent":{"maxBlockHeight":"10","maxFee":"2010000","spec":{"version":1,"sourceDomain":6,"destinationDomain":0,"value":"1000000"}}}],"fees":{"total":"2.01","token":"USDC","perIntent":[{"transferSpecHash":"0x00","domain":6,"baseFee":"2","transferFee":"0.01"}]}}`))
	est, err := New(s.URL).Estimate(context.Background(), []Intent{{Spec: Spec{Version: 1, SourceDomain: 6, Value: "1000000"}}})
	if err != nil || s.path != "/v1/estimate" || !strings.HasPrefix(string(s.raw), "[{") {
		t.Fatalf("estimate: %v body %s", err, s.raw)
	}
	if est.Fees.Total != "2.01" || est.Body[0].BurnIntent.MaxFee != "2010000" {
		t.Fatalf("estimate = %+v", est)
	}
}

func TestIdsAreUUIDsBeforeTheyEnterAPath(t *testing.T) {
	s := serve(t, answer(200, `{"status":"finalized"}`))
	c := New(s.URL)
	for _, bad := range []string{"", "../info", "123e4567-e89b-12d3-a456-42661417400", "123e4567/e89b-12d3-a456-426614174000", "123e4567-e89b-12d3-a456-42661417400g"} {
		if _, err := c.Transfer(context.Background(), bad); err == nil {
			t.Errorf("transfer %q went out", bad)
		}
		if _, err := c.Payment(context.Background(), bad); err == nil {
			t.Errorf("payment %q went out", bad)
		}
	}
	tr, err := c.Transfer(context.Background(), "123e4567-e89b-12d3-a456-426614174000")
	if err != nil || s.path != "/v1/transfer/123e4567-e89b-12d3-a456-426614174000" || tr.Status != "finalized" {
		t.Fatalf("transfer: %+v %v %s", tr, err, s.path)
	}
}

func TestSettleSendsTheX402Shape(t *testing.T) {
	s := serve(t, answer(200, `{"success":true,"payer":"0xaaa","transaction":"123e4567-e89b-12d3-a456-426614174000","network":"eip155:8453"}`))
	req := Requirements{Scheme: Scheme, Network: "eip155:8453", Asset: "0xusdc", Amount: "1000000", PayTo: "0xtreasury",
		MaxTimeoutSeconds: 2592000, Extra: Signing{Name: Name, Version: Release, VerifyingContract: "0xwallet"}}
	auth := Authorization{From: "0xaaa", To: "0xtreasury", Value: "1000000", ValidAfter: "1", ValidBefore: "2", Nonce: "0x11"}
	st, err := New(s.URL).Settle(context.Background(), Resource{URL: "/r", Description: "d", MimeType: "application/json"}, req, Payload{auth, "0xsig"})
	if err != nil || !st.Success || st.Transaction == "" || s.path != "/v1/x402/settle" {
		t.Fatalf("settle: %+v %v", st, err)
	}
	pay := s.body["paymentPayload"].(map[string]any)
	if pay["x402Version"].(float64) != 2 {
		t.Fatalf("x402Version = %v", pay["x402Version"])
	}
	res := pay["resource"].(map[string]any)
	if res["url"] != "/r" || res["description"] != "d" || res["mimeType"] != "application/json" {
		t.Fatalf("resource = %v (Gateway requires all three)", res)
	}
	inner := pay["payload"].(map[string]any)
	if inner["signature"] != "0xsig" || inner["authorization"].(map[string]any)["nonce"] != "0x11" {
		t.Fatalf("payload = %v", inner)
	}
	for _, r := range []map[string]any{pay["accepted"].(map[string]any), s.body["paymentRequirements"].(map[string]any)} {
		extra := r["extra"].(map[string]any)
		if r["payTo"] != "0xtreasury" || r["amount"] != "1000000" || r["scheme"] != "exact" ||
			extra["name"] != "GatewayWalletBatched" || extra["version"] != "1" || extra["verifyingContract"] != "0xwallet" {
			t.Fatalf("requirements = %v", r)
		}
	}
}

func TestSettleRefusalIsAnAnswerNotAnError(t *testing.T) {
	s := serve(t, answer(200, `{"success":false,"errorReason":"insufficient_balance","transaction":"","network":"eip155:8453"}`))
	st, err := New(s.URL).Settle(context.Background(), Resource{}, Requirements{}, Payload{})
	if err != nil || st.Success || st.Reason != "insufficient_balance" {
		t.Fatalf("settle: %+v %v", st, err)
	}
}

// Only a 4xx that is not a timeout or a throttle means Gateway did nothing.
func TestOnlyADefinitiveRefusalIsRefused(t *testing.T) {
	for status, refused := range map[int]bool{400: true, 404: true, 409: true, 422: true, 408: false, 429: false, 500: false, 502: false, 503: false, 504: false, 302: false} {
		s := serve(t, answer(status, `{"code":1,"message":"nope"}`))
		_, err := New(s.URL).Settle(context.Background(), Resource{}, Requirements{}, Payload{})
		var e *Error
		if !errors.As(err, &e) || e.Status != status {
			t.Fatalf("%d: %v", status, err)
		}
		if Refused(err) != refused {
			t.Errorf("Refused(%d) = %v, want %v", status, Refused(err), refused)
		}
	}
	if e := (&Error{Status: 404, Code: 1, Message: "nope"}); e.Error() != "circle gateway: HTTP 404: nope" {
		t.Fatal(e.Error())
	}
}

func TestATransportFailureIsNotARefusal(t *testing.T) {
	s := serve(t, func(w http.ResponseWriter, r *http.Request) { time.Sleep(200 * time.Millisecond) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := New(s.URL).Settle(ctx, Resource{}, Requirements{}, Payload{})
	if err == nil || Refused(err) {
		t.Fatalf("a timeout answered %v, Refused=%v", err, Refused(err))
	}
	dead := New("http://127.0.0.1:1")
	if _, err := dead.Info(context.Background()); err == nil || Refused(err) {
		t.Fatalf("a refused connection answered %v", err)
	}
}

func TestAnUnreadableSuccessIsAnError(t *testing.T) {
	s := serve(t, answer(200, `<html>maintenance</html>`))
	_, err := New(s.URL).Settle(context.Background(), Resource{}, Requirements{}, Payload{})
	var e *Error
	if !errors.As(err, &e) || e.Status != 200 || Refused(err) {
		t.Fatalf("an unreadable 200 answered %v", err)
	}
}

func TestAnOversizedBodyIsRefusedUnread(t *testing.T) {
	big := `{"kinds":[],"pad":"` + strings.Repeat("x", limit) + `"}`
	s := serve(t, answer(200, big))
	if _, err := New(s.URL).Supported(context.Background()); err == nil {
		t.Fatal("a body past the limit was read")
	}
}

// A redirect is not followed: a signed payment goes to the host the operator
// named and nowhere else.
func TestARedirectIsNotFollowed(t *testing.T) {
	var hit bool
	elsewhere := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hit = true }))
	defer elsewhere.Close()
	s := serve(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+"/v1/x402/settle", http.StatusTemporaryRedirect)
	})
	_, err := New(s.URL).Settle(context.Background(), Resource{}, Requirements{}, Payload{})
	if err == nil || hit {
		t.Fatalf("redirect followed (hit=%v, err=%v)", hit, err)
	}
}

func TestFindAsksForOnePayersNonceOnOneNetwork(t *testing.T) {
	s := serve(t, answer(200, `{"transfers":[{"id":"123e4567-e89b-12d3-a456-426614174000","status":"batched","fromAddress":"0xaaa","toAddress":"0xbbb","amount":"5","nonce":"0x11","txHash":null}]}`))
	pays, err := New(s.URL).Find(context.Background(), "eip155:8453", "0xaaa", "0x11")
	if err != nil || len(pays) != 1 || pays[0].Status != Batched || pays[0].TxHash != "" {
		t.Fatalf("find: %+v %v", pays, err)
	}
	if s.path != "/v1/x402/transfers" || !strings.Contains(s.query, "network=eip155:8453") ||
		!strings.Contains(s.query, "from=0xaaa") || !strings.Contains(s.query, "nonce=0x11") {
		t.Fatalf("query = %s", s.query)
	}
}

func TestSupportedReadsTheLiveShape(t *testing.T) {
	// Trimmed from GET https://gateway-api-testnet.circle.com/v1/x402/supported (2026-10-26).
	s := serve(t, answer(200, `{"kinds":[{"x402Version":2,"scheme":"exact","network":"eip155:84532","extra":{"name":"GatewayWalletBatched","version":"1","verifyingContract":"0x0077777d7eba4688bdef3e311b846f25870a19b9","minValiditySeconds":604800,"assets":[{"symbol":"USDC","address":"0x036cbd53842c5426634e7929541ec2318f3dcf7e","decimals":6,"authorizationVersions":["1","2"]},{"symbol":"EURC","address":"0x808456652fdb597867f38412077a9182bf77359f","decimals":6,"authorizationVersions":["2"]}]}}],"extensions":[],"signers":{"eip155:*":[]}}`))
	kinds, err := New(s.URL).Supported(context.Background())
	if err != nil || len(kinds) != 1 {
		t.Fatalf("supported: %v", err)
	}
	k := kinds[0]
	if k.Network != "eip155:84532" || k.Extra.Name != Name || k.Extra.Version != Release || k.Extra.MinValidity != 604800 ||
		len(k.Extra.Assets) != 2 || k.Extra.Assets[0].Versions[0] != "1" {
		t.Fatalf("kind = %+v", k)
	}
}

func TestFromEnv(t *testing.T) {
	treasury := "0x7E57000000000000000000000000000000000001"
	cfg, err := FromEnv([]string{EnvTreasury + "=" + treasury, EnvNetwork + "=production"})
	if err != nil || cfg.Network != Mainnet || cfg.Treasury != strings.ToLower(treasury) || !cfg.Armed() {
		t.Fatalf("production: %+v %v", cfg, err)
	}
	cfg, err = FromEnv([]string{EnvTreasury + "= " + treasury + " ", EnvNetwork + "=sandbox"})
	if err != nil || cfg.Network != Testnet {
		t.Fatalf("sandbox: %+v %v", cfg, err)
	}
	// Unarmed: no treasury, whatever else is set.
	for _, env := range [][]string{nil, {EnvNetwork + "=production"}, {EnvTreasury + "="}} {
		cfg, err := FromEnv(env)
		if err != nil || cfg.Armed() {
			t.Fatalf("%v: %+v %v", env, cfg, err)
		}
	}
	// Armed, but incoherent: refused.
	for _, env := range [][]string{
		{EnvTreasury + "=" + treasury},
		{EnvTreasury + "=" + treasury, EnvNetwork + "=Production"},
		{EnvTreasury + "=" + treasury, EnvNetwork + "=mainnet"},
		{EnvTreasury + "=7e57000000000000000000000000000000000001", EnvNetwork + "=production"},
		{EnvTreasury + "=0x513f", EnvNetwork + "=production"},
		{EnvTreasury + "=0x" + strings.Repeat("0", 40), EnvNetwork + "=production"},
	} {
		if _, err := FromEnv(env); err == nil {
			t.Errorf("%v: accepted", env)
		}
	}
}
