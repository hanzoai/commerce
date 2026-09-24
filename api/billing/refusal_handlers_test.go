package billing

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/test/ae"
)

// invokeCardSave drives CreatePaymentMethod, the card-save endpoint, for org.
func invokeCardSave(org *organization.Organization, ctx context.Context, body string) *http.Response {
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/payment-methods", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return driveSeeded(func(c *zip.Ctx) {
		c.Locals("organization", org)
		c.SetContext(ctx)
	}, "/v1/billing/payment-methods", req, CreatePaymentMethod)
}

// TestRefusal_TheHandlersAnswerAHeldAttempt — commerce's own card endpoints answer a
// wallet at its ceiling 429 and a wallet with an attempt in progress 409, in their
// own sentences, before the card is tried.
func TestRefusal_TheHandlersAnswerAHeldAttempt(t *testing.T) {
	for _, held := range []struct {
		name     string
		hold     func(db *datastore.Datastore, org, subject string) func()
		status   int
		sentence string
	}{
		{"at the ceiling", func(db *datastore.Datastore, _, subject string) func() {
			for range declineCeiling {
				tally(db, subject)
			}
			return func() {}
		}, 429, ceilingSentence},
		{"with an attempt in progress", func(db *datastore.Datastore, org, subject string) func() {
			release, err := attempt(db, org, subject)
			if err != nil {
				panic(err)
			}
			return release
		}, 409, attemptSentence},
	} {
		t.Run(held.name, func(t *testing.T) {
			ctx := ae.NewContext()
			defer ctx.Close()
			name := "handlers-" + strings.ReplaceAll(held.name, " ", "-")
			org := moneyOrg(name)
			db := datastore.New(org.Namespaced(ctx))
			pm := seedSavedCard(t, db, name, "ccof_h", "cust_h")
			m := squareMock("cust_h", "ccof_h", "sqpay_h")
			withFakeSquare(t, m)
			release := held.hold(db, name, name)
			defer release()

			for endpoint, resp := range map[string]*http.Response{
				"saved-card top-up": invokeTopup(org, ctx, fmt.Sprintf(`{"userId":%q,"paymentMethodId":%q,"amountCents":2500}`, name, pm.Id()), nil),
				"plan sale":         invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:h","planId":"dev"}`, nil),
				"card save":         invokeCardSave(org, ctx, fmt.Sprintf(`{"customerId":%q,"type":"card","providerRef":"cnon:h"}`, name)),
			} {
				body, _ := io.ReadAll(resp.Body)
				if resp.StatusCode != held.status || !strings.Contains(string(body), held.sentence) {
					t.Errorf("%s: answered %d %s, want %d %q", endpoint, resp.StatusCode, body, held.status, held.sentence)
				}
			}
			if m.chargeCalls != 0 {
				t.Errorf("the card was tried %d time(s) while the wallet was held", m.chargeCalls)
			}
		})
	}
}
