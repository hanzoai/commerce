// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"context"
	"crypto/elliptic"
	"math/big"
	"strings"
	"testing"
)

// reorder reverses the disclosures of a token.
func reorder(text string) string {
	parts := strings.Split(text, "~") // jwt, d1..dn, ""
	d := parts[1 : len(parts)-1]
	for i, j := 0, len(d)-1; i < j; i, j = i+1, j-1 {
		d[i], d[j] = d[j], d[i]
	}
	return strings.Join(parts, "~")
}

// flipS makes the other valid ES256 signature, (r, n-s), of the same JWT.
func flipS(t *testing.T, jwt string) string {
	t.Helper()
	p := strings.Split(jwt, ".")
	sig, err := b64.DecodeString(p[2])
	if err != nil {
		t.Fatal(err)
	}
	s := new(big.Int).SetBytes(sig[32:])
	s.Sub(elliptic.P256().Params().N, s).FillBytes(sig[32:])
	return p[0] + "." + p[1] + "." + b64.EncodeToString(sig)
}

// An open mandate buys one checkout. The agent holds the key its closed mandates are signed
// with, and anyone who holds the presentation can re-spell it: reorder the disclosures, change
// s of the issuer's signature to n-s (which verifies), put a line feed in a segment (which
// base64 skips), or show fewer disclosures (see the id test below). None of it may make the open mandate another one.
func TestAnOpenMandateRespelledBuysNoSecondCheckout(t *testing.T) {
	s := newShop(t)
	ctx := context.Background()
	var first string
	buy := func(open func(string) string) error {
		iss := s.issue(t)
		s.world.nonce, s.world.aud = iss.Nonce, iss.Audience
		cm := s.checkoutMandate(iss.JWT, iss.Hash)
		if first == "" { // the user signs the open mandate once
			first = cm.openText()
		}
		cm.openOut = open(first)
		pay := s.rootTok(s.sd(), paymentItem(iss.Hash, 500)).text(t)
		_, err := s.m.Complete(ctx, s.who, Completion{ID: iss.ID, Checkout: cm.text(), Payment: pay, Token: "spt"})
		return err
	}
	if err := buy(func(o string) string { return o }); err != nil {
		t.Fatal(err)
	}
	// The same mandate is refused.
	if err := buy(func(o string) string { return o }); err == nil {
		t.Fatal("the same open mandate bought twice")
	}
	jwtOf := func(o string) string { return strings.SplitN(o, "~", 2)[0] }
	for name, respell := range map[string]func(string) string{
		"reordered disclosures": reorder,
		"s flipped to n-s": func(o string) string {
			flipped := flipS(t, jwtOf(o)) + "~" + strings.SplitN(o, "~", 2)[1]
			if j, err := parseJWS(jwtOf(flipped)); err != nil || j.verify(&s.root.key.PublicKey) != nil {
				t.Fatalf("the flipped signature does not verify: %v", err)
			}
			return flipped
		},
		"a line feed in the signature": func(o string) string {
			j := jwtOf(o)
			return j[:len(j)-10] + "\n" + j[len(j)-10:] + "~" + strings.SplitN(o, "~", 2)[1]
		},
		"a carriage return in a disclosure": func(o string) string {
			parts := strings.Split(o, "~")
			parts[1] = parts[1][:5] + "\r" + parts[1][5:]
			return strings.Join(parts, "~")
		},
	} {
		if err := buy(respell); err == nil {
			t.Errorf("%s: the open mandate bought another checkout (charges %d)", name, s.store.count())
		}
	}
	if s.store.count() != 1 {
		t.Fatalf("charges %d under one open mandate", s.store.count())
	}
}

// What a holder cannot change is what names the mandate.
func TestAnOpenMandateIsNamedByWhatItsIssuerSigned(t *testing.T) {
	w := newWorld(t)
	_, jwt, hash := w.checkout("chk_1", 1, 500)
	a := w.checkoutMandate(jwt, hash)
	text := a.openText()
	id := func(s string) string {
		p, err := parsePresentation(s + "~")
		if err != nil {
			t.Fatal(err)
		}
		d := p.tokens[0].id()
		return d
	}
	base := strings.TrimSuffix(text, "~")
	for name, other := range map[string]string{
		"reordered":    reorder(text),
		"s flipped":    flipS(t, strings.SplitN(text, "~", 2)[0]) + "~" + strings.SplitN(text, "~", 2)[1],
		"one withheld": strings.Join(append(strings.Split(text, "~")[:1], strings.Split(text, "~")[2:]...), "~"),
	} {
		if id(strings.TrimSuffix(other, "~")) != id(base) {
			t.Errorf("%s: the id changed", name)
		}
	}
	// ... and another mandate has another id.
	b := w.checkoutMandate(jwt, hash)
	if id(strings.TrimSuffix(b.openText(), "~")) == id(base) {
		t.Error("two mandates share an id")
	}
}

func TestBase64urlIsCanonical(t *testing.T) {
	for _, bad := range []string{"AA\nA", "AA\rA", "AA A", "AAA=", "AA+A", "AA/A", "\x00AAA", "ÀAAA"} {
		if _, err := unb64(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if raw, err := unb64("SGVsbG8"); err != nil || string(raw) != "Hello" {
		t.Fatalf("%q %v", raw, err)
	}
}
