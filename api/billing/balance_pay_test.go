package billing

import (
	"io"
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/test/ae"
)

// POST /v1/billing/subscribe/card with sourceId "balance" takes exactly the plan's
// price from the books GET /v1/billing/balance reads, or it is refused and nothing
// moves. team = 2500c per seat, two seats: 5000c.
const teamOfTwo = `{"sourceId":"balance","planId":"team","quantity":2}`
const teamPrice = 5000

func TestSubscribeCardFromBalance_DebitsExactlyThePrice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("balcard-exact")
	db := datastore.New(org.Namespaced(ctx))
	deposit(t, db, org.Name, 7000)

	resp := invokeSubscribeCard(org, ctx, teamOfTwo, nil)
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s, want 201", resp.StatusCode, b)
	}
	if got := walletOf(t, ctx, org, org.Name); got != 7000-teamPrice {
		t.Fatalf("balance after = %d, want %d", got, 7000-teamPrice)
	}
	if n := len(subsOf(t, db, org.Name)); n == 0 {
		t.Fatal("paid, and no subscription")
	}
}

func TestSubscribeCardFromBalance_OneCentShortIsRefused(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("balcard-short")
	db := datastore.New(org.Namespaced(ctx))
	deposit(t, db, org.Name, teamPrice-1)

	resp := invokeSubscribeCard(org, ctx, teamOfTwo, nil)
	if resp.StatusCode == http.StatusCreated {
		t.Fatal("a balance one cent short bought the plan")
	}
	if got := walletOf(t, ctx, org, org.Name); got != teamPrice-1 {
		t.Fatalf("balance after refusal = %d, want %d", got, teamPrice-1)
	}
	if n := len(subsOf(t, db, org.Name)); n != 0 {
		t.Fatalf("a refused sale wrote %d subscription(s)", n)
	}
}

// Embedded in cloud, the balance is the host ledger's; the draw lands there.
func TestSubscribeCardFromBalance_HostLedger(t *testing.T) {
	for _, tc := range []struct {
		name   string
		funded int64
		sold   bool
	}{
		{"exact", 6000, true},
		{"short", teamPrice - 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := ae.NewContext()
			defer ctx.Close()
			fake := newFakeLedger()
			injectLedger(t, fake)
			org := moneyOrg("balcard-host-" + tc.name)
			acct := fake.account(org.Name, org.Name, "usd", false)
			fake.bal[acct] = tc.funded

			resp := invokeSubscribeCard(org, ctx, teamOfTwo, nil)
			if sold := resp.StatusCode == http.StatusCreated; sold != tc.sold {
				b, _ := io.ReadAll(resp.Body)
				t.Fatalf("status=%d body=%s, want sold=%v", resp.StatusCode, b, tc.sold)
			}
			want, debits := tc.funded, 0
			if tc.sold {
				want, debits = tc.funded-teamPrice, 1
			}
			if fake.bal[acct] != want || len(fake.debits) != debits {
				t.Fatalf("host balance %d with %d debit(s), want %d with %d", fake.bal[acct], len(fake.debits), want, debits)
			}
			if tc.sold && (fake.debits[0].AmountCents != teamPrice || fake.debits[0].Ref == "") {
				t.Fatalf("debit = %+v, want one keyed debit of %d", fake.debits[0], teamPrice)
			}
		})
	}
}
