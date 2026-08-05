package billing

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/balancetransaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"
)

// GetCustomerBalance retrieves the customer balance for a customer+currency.
//
//	GET /v1/billing/customer-balance?customerId=...&currency=...
func GetCustomerBalance(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	customerId := c.Query("customerId")
	if customerId == "" {
		return http.Fail(c, 400, "customerId is required", nil)
	}

	curQ := c.Query("currency")
	if curQ == "" {
		curQ = "usd"
	}
	cur := currency.Type(curQ)

	cb, err := engine.GetOrCreateCustomerBalance(db, customerId, cur)
	if err != nil {
		log.Error("Failed to get customer balance: %v", err, c)
		return http.Fail(c, 500, "failed to get balance", err)
	}

	return c.JSON(200, map[string]any{
		"customerId": cb.CustomerId,
		"currency":   cb.Currency,
		"balance":    cb.Balance,
	})
}

type adjustBalanceRequest struct {
	CustomerId  string `json:"customerId"`
	Amount      int64  `json:"amount"` // positive = credit, negative = debit
	Currency    string `json:"currency,omitempty"`
	Description string `json:"description,omitempty"`
}

// AdjustCustomerBalance manually adjusts a customer's balance.
//
//	POST /v1/billing/customer-balance/adjustments
func AdjustCustomerBalance(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	var req adjustBalanceRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	if req.CustomerId == "" {
		return http.Fail(c, 400, "customerId is required", nil)
	}
	if req.Amount == 0 {
		return http.Fail(c, 400, "amount must be non-zero", nil)
	}

	cur := currency.Type("usd")
	if req.Currency != "" {
		cur = currency.Type(req.Currency)
	}

	bt, err := engine.AdjustCustomerBalance(db, req.CustomerId, req.Amount, cur, "adjustment", req.Description)
	if err != nil {
		log.Error("Failed to adjust balance: %v", err, c)
		return http.Fail(c, 500, "failed to adjust balance", err)
	}

	return c.JSON(200, balanceTransactionResponse(bt))
}

// ListBalanceTransactions lists balance transactions for a customer.
//
//	GET /v1/billing/balance-transactions?customerId=...
func ListBalanceTransactions(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	rootKey := db.NewKey("synckey", "", 1, nil)
	txns := make([]*balancetransaction.BalanceTransaction, 0)
	q := balancetransaction.Query(db).Ancestor(rootKey)

	if customerId := c.Query("customerId"); customerId != "" {
		q = q.Filter("CustomerId=", customerId)
	}

	iter := q.Order("-Created").Run()
	for {
		bt := balancetransaction.New(db)
		if _, err := iter.Next(bt); err != nil {
			break
		}
		txns = append(txns, bt)
	}

	results := make([]map[string]interface{}, len(txns))
	for i, bt := range txns {
		results[i] = balanceTransactionResponse(bt)
	}
	return c.JSON(200, results)
}

func balanceTransactionResponse(bt *balancetransaction.BalanceTransaction) map[string]interface{} {
	resp := map[string]interface{}{
		"id":            bt.Id(),
		"customerId":    bt.CustomerId,
		"amount":        bt.Amount,
		"currency":      bt.Currency,
		"type":          bt.Type,
		"endingBalance": bt.EndingBalance,
		"created":       bt.Created,
	}
	if bt.Description != "" {
		resp["description"] = bt.Description
	}
	if bt.InvoiceId != "" {
		resp["invoiceId"] = bt.InvoiceId
	}
	if bt.CreditNoteId != "" {
		resp["creditNoteId"] = bt.CreditNoteId
	}
	if bt.SourceRef != "" {
		resp["sourceRef"] = bt.SourceRef
	}
	if bt.Metadata != nil {
		resp["metadata"] = bt.Metadata
	}
	return resp
}
