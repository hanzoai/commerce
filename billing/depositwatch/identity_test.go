package depositwatch

import (
	"testing"

	"github.com/hanzoai/commerce/models/cryptopaymentintent"
)

// One payer's SECOND deposit must not stop the asset.
//
// This is the exact production failure, reproduced: `ethereum:usdc … claimed by
// more than one intent`, once every pass, with the cursor frozen behind it. It
// took two upstream decisions that are each correct to produce it — Watched()
// enumerates intents regardless of status because money sent to an expired
// intent is still the customer's, and CreateCryptoDeposit reuses one address per
// (payer, chain, token) because on a per-payer chain the address IS derived from
// that payer's key. So a customer's second deposit necessarily leaves two
// intents on one address, and counting them read that as a collision.
func TestSamePayerTwiceIsNotAmbiguous(t *testing.T) {
	a := &asset{Asset: Asset{Chain: "ethereum", Token: "usdc"}}
	const addr = "0x6d920BE01d52f5D749958c0a4c6bF977b9E05DB4"

	watched := []Watched{
		{Org: "acme", IntentID: "int_001", Subject: "user_1", Address: addr, Status: cryptopaymentintent.Succeeded},
		{Org: "acme", IntentID: "int_002", Subject: "user_1", Address: addr, Status: cryptopaymentintent.Pending},
	}

	byID, addrs, ambiguous := indexWatched(watched, a)

	if len(ambiguous) != 0 {
		t.Fatalf("one payer's own two intents reported ambiguous %v — this halts the whole asset", ambiguous)
	}
	if len(addrs) != 1 {
		t.Fatalf("addresses = %v, want the one address watched exactly once", addrs)
	}
	got, ok := byID[a.Identity(addr, "")]
	if !ok {
		t.Fatal("the identity resolves to no intent, so a deposit to it would be seen and then dropped")
	}
	// The open intent wins: that is the one the customer has on screen.
	if got.IntentID != "int_002" {
		t.Fatalf("credited intent = %s, want int_002 (the open one)", got.IntentID)
	}
}

// Two DIFFERENT payers on one identity is the real collision, and it must still
// fail closed. Crediting either is a guess with someone's money.
func TestDifferentPayersStillAmbiguous(t *testing.T) {
	a := &asset{Asset: Asset{Chain: "ethereum", Token: "usdc"}}
	const addr = "0xe66Da5019005F15B002d313f183d8f779AFe5346"

	watched := []Watched{
		{Org: "acme", IntentID: "int_001", Subject: "user_1", Address: addr, Status: cryptopaymentintent.Pending},
		{Org: "acme", IntentID: "int_002", Subject: "user_2", Address: addr, Status: cryptopaymentintent.Pending},
	}

	byID, addrs, ambiguous := indexWatched(watched, a)

	if len(ambiguous) != 1 {
		t.Fatalf("two payers on one identity reported ambiguous=%v — someone would be credited another person's deposit", ambiguous)
	}
	if _, ok := byID[a.Identity(addr, "")]; ok {
		t.Fatal("an ambiguous identity still resolves to an intent")
	}
	// The address stays watched even while ambiguous: not asking the chain about
	// an address we own is how money arrives unseen.
	if len(addrs) != 1 {
		t.Fatalf("addresses = %v, want the address still watched", addrs)
	}
}

// A pooled chain gives each payer its own TAG on one shared address, so two
// payers there are two identities and neither is ambiguous. The guard must not
// confuse "one address" with "one identity".
func TestPooledChainTagsSeparatePayers(t *testing.T) {
	a := &asset{Asset: xrplAsset()} // pooled is derived from the chain
	const shared = "rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De"

	watched := []Watched{
		{Org: "acme", IntentID: "int_001", Subject: "user_1", Address: shared, Tag: "1001", Status: cryptopaymentintent.Pending},
		{Org: "acme", IntentID: "int_002", Subject: "user_2", Address: shared, Tag: "1002", Status: cryptopaymentintent.Pending},
	}

	byID, addrs, ambiguous := indexWatched(watched, a)

	if len(ambiguous) != 0 {
		t.Fatalf("distinct tags reported ambiguous %v — the pooled rail would stop", ambiguous)
	}
	if len(byID) != 2 {
		t.Fatalf("identities resolved = %d, want 2 (one per tag)", len(byID))
	}
	if len(addrs) != 1 {
		t.Fatalf("addresses = %v, want the one pooled address asked about once", addrs)
	}
}

// Determinism: orgs enumerate in whatever order the datastore returns, and the
// chosen intent must not depend on it.
func TestChoiceIsOrderIndependent(t *testing.T) {
	a := &asset{Asset: Asset{Chain: "ethereum", Token: "usdc"}}
	const addr = "0x6d920BE01d52f5D749958c0a4c6bF977b9E05DB4"

	forward := []Watched{
		{IntentID: "int_001", Subject: "u", Address: addr, Status: cryptopaymentintent.Succeeded},
		{IntentID: "int_002", Subject: "u", Address: addr, Status: cryptopaymentintent.Succeeded},
	}
	reversed := []Watched{forward[1], forward[0]}

	f, _, _ := indexWatched(forward, a)
	r, _, _ := indexWatched(reversed, a)

	id := a.Identity(addr, "")
	if f[id].IntentID != r[id].IntentID {
		t.Fatalf("enumeration order changed the credited intent: %s vs %s", f[id].IntentID, r[id].IntentID)
	}
}
