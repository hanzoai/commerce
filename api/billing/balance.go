package billing

import (
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"
)

// GetBalance returns the current balance for an IAM user.
//
//	GET /v1/billing/balance?user=hanzo/alice&currency=usd
//
// All amounts in cents. available = balance - holds.
func GetBalance(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c.Context())

	user := strings.ToLower(strings.TrimSpace(c.Query("user")))
	if user == "" {
		return http.Fail(c, 400, "user query parameter is required", nil)
	}

	cur := currency.Type(strings.ToLower(c.Query("currency")))
	if cur == "" {
		cur = "usd"
	}

	// Injected double-entry ledger (embedded-in-cloud path): the balance is
	// authoritative from the SAME account POST /billing/credit writes and the AI gate
	// reads — one ledger by construction.
	//
	// THE ORG AND THE SUBJECT ARE TWO VALUES, and passing one for both is what this
	// fixes. `user` is a SUBJECT — the org's pool key, or the finer "<org>/<user>" of
	// a member who spends from their own wallet, which is exactly what the datastore
	// path below hands bucketedSplit and what this door's own example documents. It
	// was being passed as the LEDGER, so a person-scoped subject opened a namespace
	// named after the person: a different ledger, holding nothing, reported as a real
	// zero to a customer with money. The org is the request's own, the same one the
	// datastore path reads, and the subject is the subject.
	if led := creditledger.Get(); led != nil {
		avail, err := led.Balance(c.Context(), strings.ToLower(strings.TrimSpace(org.Name)), user, string(cur), org.TestMode())
		if err != nil {
			return http.Fail(c, 500, "failed to query balance", err)
		}
		return c.JSON(200, map[string]any{
			"user":      user,
			"currency":  cur,
			"balance":   avail,
			"holds":     int64(0),
			"available": avail,
		})
	}

	// The three-bucket split (credits granted/remaining vs prepaid real money) is
	// derived from the SAME ledger the balance is; balance/holds/available come
	// straight from the split so a bucketed read reconciles to the cent and stays
	// backward compatible with the pre-split {balance,holds,available} shape.
	split, err := bucketedSplit(ctx, user, cur, org.TestMode())
	if err != nil {
		return http.Fail(c, 500, "failed to query balance", err)
	}
	card := getCardOnFile(datastore.New(ctx), user)

	resp := map[string]any{
		"user":      user,
		"currency":  cur,
		"balance":   int64(split.Balance),
		"holds":     int64(split.Holds),
		"available": int64(split.Available),
	}
	for k, v := range bucketFields(split, card) {
		resp[k] = v
	}
	return c.JSON(200, resp)
}

// GetBalanceAll returns balances across all currencies for an IAM user.
//
//	GET /v1/billing/balance/all?user=hanzo/alice
func GetBalanceAll(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c.Context())

	user := strings.ToLower(strings.TrimSpace(c.Query("user")))
	if user == "" {
		return http.Fail(c, 400, "user query parameter is required", nil)
	}

	datas, err := util.GetTransactions(ctx, user, "iam-user", org.TestMode())
	if err != nil {
		return http.Fail(c, 500, "failed to query balance", err)
	}

	balances := make([]map[string]any, 0, len(datas.Data))
	for cur, data := range datas.Data {
		available := data.Balance - data.Holds
		if available < 0 {
			available = 0
		}
		balances = append(balances, map[string]any{
			"currency":  cur,
			"balance":   data.Balance,
			"holds":     data.Holds,
			"available": available,
		})
	}

	return c.JSON(200, map[string]any{
		"user":     user,
		"balances": balances,
	})
}
