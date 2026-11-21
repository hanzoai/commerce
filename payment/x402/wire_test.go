// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package x402

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func read(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

// sameJSON fails unless a and b are the same JSON value.
func sameJSON(t *testing.T, a, b []byte) {
	t.Helper()
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &y); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(x, y) {
		t.Fatalf("JSON differs:\n got %s\nwant %s", a, b)
	}
}

// Each header the specification prints decodes to the JSON the specification
// says it decodes to, and our types carry every field of it: encoding the
// decoded value back gives the same document.
func TestSpecHeadersRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		into any
	}{
		{"payment-required", &PaymentRequired{}},
		{"payment-signature", &PaymentPayload{}},
		{"payment-response", &SettlementResponse{}},
		{"payment-response-failure", &SettlementResponse{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := DecodeHeader(read(t, c.name+".b64"), c.into); err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(c.into)
			if err != nil {
				t.Fatal(err)
			}
			sameJSON(t, got, []byte(read(t, c.name+".json")))
		})
	}
}

// The spec's example payment is a real signature: it recovers to the payer the
// example names, under the token domain the example offers. This is the
// protocol's own vector, independent of this package's encoder.
func TestSpecSignatureRecovers(t *testing.T) {
	var required PaymentRequired
	if err := DecodeHeader(read(t, "payment-required.b64"), &required); err != nil {
		t.Fatal(err)
	}
	pay, err := ParsePayment(read(t, "payment-signature.b64"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(required.Accepts[0], pay); err != nil {
		t.Fatalf("the spec's own payment does not verify: %v", err)
	}
	// Its window is long past: refused as expired, for the spec's reason.
	if err := InWindow(pay.Payload.Authorization, time.Unix(1740672154, 0)); ReasonOf(err, "") != ReasonValidBefore {
		t.Fatalf("at validBefore: %v", err)
	}
	if err := InWindow(pay.Payload.Authorization, time.Unix(1740672089, 0)); ReasonOf(err, "") != ReasonValidAfter {
		t.Fatalf("at validAfter: %v", err)
	}
	if err := InWindow(pay.Payload.Authorization, time.Unix(1740672100, 0)); err != nil {
		t.Fatalf("inside the window: %v", err)
	}
}

func TestEncodeHeaderIsPaddedStandardBase64(t *testing.T) {
	var s SettlementResponse
	raw := read(t, "payment-response.b64")
	if err := DecodeHeader(raw, &s); err != nil {
		t.Fatal(err)
	}
	got, err := EncodeHeader(s)
	if err != nil {
		t.Fatal(err)
	}
	var back SettlementResponse
	if err := DecodeHeader(got, &back); err != nil || !reflect.DeepEqual(back, s) {
		t.Fatalf("round trip: %v %+v", err, back)
	}
	if strings.ContainsAny(got, "-_") {
		t.Fatalf("not standard base64: %s", got)
	}
}

func TestDecodeHeaderAcceptsTheOtherBase64Spellings(t *testing.T) {
	raw := read(t, "payment-response.b64")
	var want SettlementResponse
	if err := DecodeHeader(raw, &want); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{strings.TrimRight(raw, "="), strings.NewReplacer("+", "-", "/", "_").Replace(raw)} {
		var got SettlementResponse
		if err := DecodeHeader(v, &got); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%q: %v", v, err)
		}
	}
	for _, bad := range []string{"", "   ", "!!!", "bm90IGpzb24="} { // the last is "not json"
		if err := DecodeHeader(bad, &want); err == nil {
			t.Errorf("%q decoded", bad)
		}
	}
}

func TestParsePaymentRequiresTheFieldsVerificationReads(t *testing.T) {
	cases := map[string]string{
		"not base64": "!!!",
		"empty":      "e30=", // {}
	}
	for name, h := range cases {
		if _, err := ParsePayment(h); ReasonOf(err, "") != ReasonInvalidPayload {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestExtensionsSurviveVerbatim(t *testing.T) {
	in := `{"x402Version":2,"resource":{"url":"u"},"accepts":[],"extensions":{"bazaar":{"info":{"a":1},"schema":{"type":"object"}}}}`
	var p PaymentRequired
	if err := json.Unmarshal([]byte(in), &p); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, out, []byte(in))
}

func TestChainID(t *testing.T) {
	good := map[string]int64{"eip155:8453": 8453, "eip155:84532": 84532, " eip155:1 ": 1}
	for n, want := range good {
		if got, err := ChainID(n); err != nil || got != want {
			t.Errorf("%q: %d %v", n, got, err)
		}
	}
	for _, n := range []string{"", "8453", "eip155", "eip155:", "eip155:0", "eip155:-1", "eip155:abc",
		"solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp", "EIP155:1", "ab:1", "eip155:" + strings.Repeat("1", 33)} {
		if _, err := ChainID(n); ReasonOf(err, "") != ReasonInvalidNetwork {
			t.Errorf("%q: %v", n, err)
		}
	}
}
