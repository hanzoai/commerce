// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func (w *world) check(text string) (*CheckoutProof, error) {
	return w.v.Checkout(context.Background(), text, Want{Audience: w.aud, Nonce: w.nonce}, &w.merchant.Key.PublicKey)
}

// flip changes a character in the middle of the first JWT's signature, so it still decodes.
func flip(text string) string {
	i := strings.Index(text, "~") - 10
	c := byte('A')
	if text[i] == 'A' {
		c = 'B'
	}
	return text[:i] + string(c) + text[i+1:]
}

func refusedWith(t *testing.T, err error, code Code, reason error) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("err = %v; want a refusal %s", err, code)
	}
	if e.Code != code || !errors.Is(err, reason) {
		t.Fatalf("err = %v; want %s / %v", err, code, reason)
	}
}

func TestAutonomousCheckoutMandateVerifies(t *testing.T) {
	w := newWorld(t)
	co, jwt, hash := w.checkout("chk_1", 1, 500)
	a := w.checkoutMandate(jwt, hash)
	text := a.text()
	p, err := w.check(text)
	if err != nil {
		t.Fatal(err)
	}
	closed, _ := parsePresentation(text)
	ref, _ := closed.reference()
	if p.Reference != ref || p.Mandate.CheckoutHash != hash || p.Checkout.ID != co.ID || p.OpenToken != a.openText() {
		t.Fatalf("proof = %+v", p)
	}
	// The receipt reference is the hash of the final SD-JWT, disclosures included.
	if want, _ := digest("", closed.last().text()); p.Reference != want {
		t.Fatalf("reference %s, want %s", p.Reference, want)
	}
}

func TestDirectCheckoutMandateVerifies(t *testing.T) {
	w := newWorld(t)
	_, jwt, hash := w.checkout("chk_1", 1, 500)
	p, err := w.check(w.directCheckout(jwt, hash).text(t))
	if err != nil || p.Open != nil || p.OpenToken != "" || p.Checkout.ID != "chk_1" {
		t.Fatalf("proof = %+v %v", p, err)
	}
}

// Every check of the chain has a mandate that fails it.
func TestChainRefusals(t *testing.T) {
	other := newParty(t, "other")
	for _, tc := range []struct {
		name   string
		change func(w *world, a *autonomous) string // the presentation, after the change
		code   Code
		reason error
	}{
		{"first token signature", func(w *world, a *autonomous) string { return flip(a.text()) }, InvalidCredential, ErrSignature},
		{"first token from an untrusted key", func(w *world, a *autonomous) string { a.open.key = other.key; return a.text() }, InvalidCredential, ErrSignature},
		{"first token with an unknown kid", func(w *world, a *autonomous) string { a.open.hdr.Kid = "unknown"; return a.text() }, InvalidCredential, ErrKey},
		{"first token with no kid", func(w *world, a *autonomous) string { a.open.hdr.Kid = ""; return a.text() }, InvalidCredential, ErrKey},
		{"first token that is key-bound", func(w *world, a *autonomous) string { a.open.hdr.Typ = "kb+sd-jwt"; return a.text() }, InvalidCredential, ErrChain},
		{"closed token signed by another key", func(w *world, a *autonomous) string { a.closed.key = other.key; return a.text() }, InvalidCredential, ErrSignature},
		{"closed token with the wrong typ", func(w *world, a *autonomous) string { a.closed.hdr.Typ = "example+sd-jwt"; return a.text() }, InvalidCredential, ErrChain},
		{"closed token with an intermediate typ", func(w *world, a *autonomous) string { a.closed.hdr.Typ = "kb+sd-jwt+kb"; return a.text() }, InvalidCredential, ErrChain},
		{"sd_hash of another token", func(w *world, a *autonomous) string {
			a.bind = func(c map[string]any, _ string) { c["sd_hash"] = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" }
			return a.text()
		}, InvalidCredential, ErrBinding},
		{"no sd_hash", func(w *world, a *autonomous) string { a.bind = func(map[string]any, string) {}; return a.text() }, InvalidCredential, ErrBinding},
		{"issuer_jwt_hash in place of sd_hash", func(w *world, a *autonomous) string {
			a.bind = func(c map[string]any, open string) {
				h, _ := digest("", strings.SplitN(open, "~", 2)[0])
				c["issuer_jwt_hash"] = h
			}
			return a.text()
		}, InvalidCredential, ErrBinding},
		{"a disclosure of the open mandate withheld after the binding", func(w *world, a *autonomous) string {
			text := a.text()
			return strings.Replace(text, "~"+a.b1.discs[0], "", 1)
		}, InvalidCredential, ErrBinding},
		{"another open mandate", func(w *world, a *autonomous) string {
			// the closed token is bound to the first open mandate; the presentation carries a second
			first := a.text()
			a.constraints(func(b *sd) []any { return []any{w.lineItems(b, 1, Item{"credit-500", "x"})} })
			second := a.openText()
			return second + "~" + first[strings.Index(first, "~~")+2:]
		}, InvalidCredential, ErrBinding},
		{"no aud", func(w *world, a *autonomous) string { delete(a.closed.claims, "aud"); return a.text() }, InvalidCredential, ErrAudience},
		{"wrong aud", func(w *world, a *autonomous) string { a.closed.claims["aud"] = "elsewhere"; return a.text() }, InvalidCredential, ErrAudience},
		{"no nonce", func(w *world, a *autonomous) string { delete(a.closed.claims, "nonce"); return a.text() }, InvalidCredential, ErrNonce},
		{"wrong nonce", func(w *world, a *autonomous) string { a.closed.claims["nonce"] = "replayed"; return a.text() }, InvalidCredential, ErrNonce},
		{"no iat", func(w *world, a *autonomous) string { delete(a.closed.claims, "iat"); return a.text() }, InvalidCredential, ErrMalformed},
		{"iat in the future", func(w *world, a *autonomous) string { a.closed.claims["iat"] = w.unix(time.Hour); return a.text() }, InvalidCredential, ErrNotYet},
		{"closed mandate expired", func(w *world, a *autonomous) string { a.closedItem["exp"] = w.unix(-time.Second); return a.text() }, InvalidCredential, ErrExpired},
		{"closed mandate not yet valid", func(w *world, a *autonomous) string { a.closedItem["nbf"] = w.unix(time.Minute); return a.text() }, InvalidCredential, ErrNotYet},
		{"open mandate expired", func(w *world, a *autonomous) string { a.openItem["exp"] = w.unix(-time.Second); return a.text() }, InvalidCredential, ErrExpired},
		{"open mandate with no cnf", func(w *world, a *autonomous) string { delete(a.openItem, "cnf"); return a.text() }, InvalidCredential, ErrChain},
		{"open mandate with a cnf key that has a private part", func(w *world, a *autonomous) string {
			a.openItem["cnf"].(map[string]any)["jwk"].(map[string]any)["d"] = "AAAA"
			return a.text()
		}, InvalidCredential, ErrKey},
		{"open mandate with a cnf key off the curve", func(w *world, a *autonomous) string {
			a.openItem["cnf"].(map[string]any)["jwk"].(map[string]any)["y"] = a.openItem["cnf"].(map[string]any)["jwk"].(map[string]any)["x"]
			return a.text()
		}, InvalidCredential, ErrKey},
		{"open mandate with a key meant for encryption", func(w *world, a *autonomous) string {
			a.openItem["cnf"].(map[string]any)["jwk"].(map[string]any)["use"] = "enc"
			return a.text()
		}, InvalidCredential, ErrKey},
		{"closed mandate carrying cnf", func(w *world, a *autonomous) string { a.closedItem["cnf"] = w.agent.cnf(); return a.text() }, InvalidCredential, ErrChain},
		{"closed mandate that is open", func(w *world, a *autonomous) string { a.closedItem["vct"] = VCTCheckoutOpen; return a.text() }, InvalidMandate, ErrType},
		{"closed mandate of the next version", func(w *world, a *autonomous) string { a.closedItem["vct"] = "mandate.checkout.2"; return a.text() }, InvalidMandate, ErrType},
		{"closed mandate of the payment type", func(w *world, a *autonomous) string { a.closedItem["vct"] = VCTPayment; return a.text() }, InvalidMandate, ErrType},
		{"open mandate of the payment type", func(w *world, a *autonomous) string { a.openItem["vct"] = VCTPaymentOpen; return a.text() }, InvalidMandate, ErrType},
		{"unreferenced disclosure", func(w *world, a *autonomous) string {
			a.b2.add(t, "extra", "value")
			a.n2++
			return a.text()
		}, InvalidCredential, ErrDisclosure},
		{"checkout_jwt withheld", func(w *world, a *autonomous) string {
			a.b2.discs = nil
			a.n2 = 0
			delete(a.closedItem, "_sd")
			return a.text()
		}, InvalidMandate, ErrMandate},
		{"checkout_hash of another checkout", func(w *world, a *autonomous) string {
			a.closedItem["checkout_hash"] = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
			return a.text()
		}, InvalidMandate, ErrHash},
		{"no checkout_hash", func(w *world, a *autonomous) string { delete(a.closedItem, "checkout_hash"); return a.text() }, InvalidMandate, ErrMandate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			_, jwt, hash := w.checkout("chk_1", 1, 500)
			text := tc.change(w, w.checkoutMandate(jwt, hash))
			_, err := w.check(text)
			refusedWith(t, err, tc.code, tc.reason)
		})
	}
}

func TestPresentationShapeRefusals(t *testing.T) {
	w := newWorld(t)
	_, jwt, hash := w.checkout("chk_1", 1, 500)
	good := w.checkoutMandate(jwt, hash).text()
	for name, text := range map[string]string{
		"empty":                        "",
		"no trailing tilde":            strings.TrimSuffix(good, "~"),
		"a trailing key-binding JWT":   good + good[:strings.Index(good, "~")],
		"three tokens":                 good + "~" + good,
		"a join with nothing after":    good + "~",
		"a token with no JWT":          "~" + good,
		"not a JWS":                    "abc~",
		"header that is not JSON":      "AAAA.AAAA.AAAA~",
		"larger than the bound":        strings.Repeat("a", maxPresentation+1),
		"a disclosure that is not b64": strings.Replace(good, "~", "~!!!!~", 1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := w.check(text)
			var e *Error
			if !errors.As(err, &e) || e.Code != InvalidCredential {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestAlgorithmsOtherThanES256AreRefused(t *testing.T) {
	w := newWorld(t)
	_, jwt, hash := w.checkout("chk_1", 1, 500)
	good := w.directCheckout(jwt, hash).text(t)
	parts := strings.SplitN(good, ".", 2)
	for _, h := range []string{
		`{"alg":"none","kid":"agent-provider-key-1"}`, `{"alg":"HS256","kid":"agent-provider-key-1"}`,
		`{"alg":"ES384","kid":"agent-provider-key-1"}`, `{"alg":"EdDSA","kid":"agent-provider-key-1"}`,
		`{"alg":"ES256","kid":"agent-provider-key-1","crit":["x"],"x":1}`,
	} {
		_, err := w.check(b64.EncodeToString([]byte(h)) + "." + parts[1])
		refusedWith(t, err, InvalidCredential, ErrAlgorithm)
	}
}

func TestDisclosureRefusals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(w *world, a *autonomous)
	}{
		{"a digest twice", func(w *world, a *autonomous) {
			d := a.openItem["constraints"].([]any)[0].(map[string]any)["items"].([]any)[0].(map[string]any)["acceptable_items"].([]any)
			a.openItem["constraints"].([]any)[0].(map[string]any)["items"].([]any)[0].(map[string]any)["acceptable_items"] = append(d, d[0])
		}},
		{"a disclosure that overwrites a claim", func(w *world, a *autonomous) {
			a.closedItem["_sd"] = append(a.closedItem["_sd"].([]any), a.b2.prop(t, "vct", "mandate.checkout.1"))
			a.n2 = len(a.b2.discs)
		}},
		{"a disclosure that names _sd", func(w *world, a *autonomous) {
			a.closedItem["_sd"] = append(a.closedItem["_sd"].([]any), a.b2.prop(t, "_sd", []any{}))
			a.n2 = len(a.b2.discs)
		}},
		{"an array disclosure where a member belongs", func(w *world, a *autonomous) {
			a.closedItem["_sd"] = append(a.closedItem["_sd"].([]any), a.b2.add(t, "only-a-value"))
			a.n2 = len(a.b2.discs)
		}},
		{"an array element with more than the digest", func(w *world, a *autonomous) {
			a.openItem["constraints"].([]any)[0].(map[string]any)["items"].([]any)[0].(map[string]any)["acceptable_items"] = []any{map[string]any{"...": "x", "id": "credit-500"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			_, jwt, hash := w.checkout("chk_1", 1, 500)
			a := w.checkoutMandate(jwt, hash)
			tc.change(w, a)
			_, err := w.check(a.text())
			refusedWith(t, err, InvalidCredential, ErrDisclosure)
		})
	}
}

func TestADisclosureSentTwiceIsRefused(t *testing.T) {
	w := newWorld(t)
	_, jwt, hash := w.checkout("chk_1", 1, 500)
	a := w.checkoutMandate(jwt, hash)
	text := a.text()
	// the same disclosure sent twice in the open token
	dup := strings.Replace(text, "~"+a.b1.discs[0]+"~", "~"+a.b1.discs[0]+"~"+a.b1.discs[0]+"~", 1)
	_, err := w.check(dup)
	refusedWith(t, err, InvalidCredential, ErrDisclosure)
}

func TestRepeatedJSONMembersAreRefused(t *testing.T) {
	for _, bad := range []string{`{"a":1,"a":2}`, `{"a":{"b":1,"b":2}}`, `[{"x":1,"x":1}]`, `{"a":1} {"b":2}`, `{"a":`, `{"a":1}x`} {
		if _, err := parse([]byte(bad)); err == nil {
			t.Errorf("%s accepted", bad)
		}
	}
	if _, err := parse([]byte(strings.Repeat("[", maxDepth+2) + strings.Repeat("]", maxDepth+2))); err == nil {
		t.Error("deep nesting accepted")
	}
}

func TestSkewIsTheVerifiersToGive(t *testing.T) {
	w := newWorld(t)
	_, jwt, hash := w.checkout("chk_1", 1, 500)
	a := w.checkoutMandate(jwt, hash)
	a.closedItem["exp"] = w.unix(-30 * time.Second)
	text := a.text()
	if _, err := w.check(text); !errors.Is(err, ErrExpired) {
		t.Fatalf("err = %v", err)
	}
	w.v.Skew = time.Minute
	if _, err := w.check(text); err != nil {
		t.Fatalf("within the skew: %v", err)
	}
}

func TestAnOpenMandateAloneAuthorizesNothing(t *testing.T) {
	w := newWorld(t)
	_, jwt, hash := w.checkout("chk_1", 1, 500)
	a := w.checkoutMandate(jwt, hash)
	_, err := w.check(a.openText())
	refusedWith(t, err, InvalidCredential, ErrChain)
}

func TestACheckoutWantsItsAudienceAndNonce(t *testing.T) {
	w := newWorld(t)
	_, jwt, hash := w.checkout("chk_1", 1, 500)
	text := w.checkoutMandate(jwt, hash).text()
	for _, want := range []Want{{}, {Audience: "merchant"}, {Nonce: "n"}} {
		if _, err := w.v.Checkout(context.Background(), text, want, &w.merchant.Key.PublicKey); err == nil {
			t.Errorf("%+v accepted", want)
		}
	}
	if _, err := w.v.Checkout(context.Background(), text, Want{Audience: "merchant", Nonce: "nonce-1"}, nil); err == nil {
		t.Error("no merchant key accepted")
	}
}

// What the merchant signed is what the mandate must hash.
func TestTheCheckoutInsideTheMandate(t *testing.T) {
	other := newParty(t, "not-the-merchant")
	for _, tc := range []struct {
		name   string
		change func(w *world) (string, string) // the checkout_jwt and its hash
		reason error
	}{
		{"signed by another key", func(w *world) (string, string) {
			forged, _ := Signer{Key: other.key, Kid: "merchant-key-1"}.Sign(Checkout{})
			h, _ := digest("", forged)
			return forged, h
		}, ErrCheckout},
		{"not a JWT", func(w *world) (string, string) { h, _ := digest("", "nope"); return "nope", h }, ErrCheckout},
		{"a checkout that is not a checkout", func(w *world) (string, string) {
			j, _ := w.merchant.Sign(map[string]any{"id": "x"})
			h, _ := digest("", j)
			return j, h
		}, ErrCheckout},
		{"a checkout with two totals", func(w *world) (string, string) {
			co, _, _ := w.checkout("chk_1", 1, 500)
			co.Totals = append(co.Totals, Total{Type: "total", Amount: 1})
			j, _ := w.merchant.Sign(co)
			h, _ := digest("", j)
			return j, h
		}, ErrCheckout},
		{"a checkout past its expiry", func(w *world) (string, string) {
			co, _, _ := w.checkout("chk_1", 1, 500)
			co.ExpiresAt = w.now.Add(-time.Second).UTC().Format(time.RFC3339)
			j, _ := w.merchant.Sign(co)
			h, _ := digest("", j)
			return j, h
		}, ErrExpired},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			jwt, hash := tc.change(w)
			_, err := w.check(w.checkoutMandate(jwt, hash).text())
			refusedWith(t, err, InvalidMandate, tc.reason)
		})
	}
}

func TestEveryHashAlgorithmTheSpecNamesWorks(t *testing.T) {
	for _, alg := range []string{"sha-256", "sha-384", "sha-512"} {
		t.Run(alg, func(t *testing.T) {
			w := newWorld(t)
			w.alg = alg
			_, jwt, hash := w.checkout("chk_1", 1, 500)
			if _, err := w.check(w.checkoutMandate(jwt, hash).text()); err != nil {
				t.Fatal(err)
			}
			// the checkout hash is in the mandate's algorithm: another algorithm's is refused
			other := "sha-512"
			if alg == "sha-512" {
				other = "sha-256"
			}
			wrong, _ := digest(other, jwt)
			_, err := w.check(w.checkoutMandate(jwt, wrong).text())
			refusedWith(t, err, InvalidMandate, ErrHash)
		})
	}
	w := newWorld(t)
	w.alg = "md5"
	_, jwt, hash := w.checkout("chk_1", 1, 500)
	_, err := w.check(w.checkoutMandate(jwt, hash).text())
	refusedWith(t, err, InvalidCredential, ErrAlgorithm)
}
