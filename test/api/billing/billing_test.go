package test

import (
	. "github.com/hanzoai/commerce/util/test/ginclient"
	. "github.com/hanzoai/commerce/util/test/ginkgo"
)

// Response types for JSON parsing
type meterResponse struct {
	Id              string   `json:"id"`
	Name            string   `json:"name"`
	EventName       string   `json:"eventName"`
	AggregationType string   `json:"aggregationType"`
	Currency        string   `json:"currency"`
	Dimensions      []string `json:"dimensions"`
}

type meterListResponse struct {
	Meters []meterResponse `json:"meters"`
	Count  int             `json:"count"`
}

type eventResult struct {
	Id     string `json:"id"`
	Status string `json:"status"`
}

type eventsResponse struct {
	Events []eventResult `json:"events"`
	Count  int           `json:"count"`
}

type summaryResponse struct {
	MeterId         string `json:"meterId"`
	MeterName       string `json:"meterName"`
	UserId          string `json:"userId"`
	AggregationType string `json:"aggregationType"`
	Value           int64  `json:"value"`
	EventCount      int64  `json:"eventCount"`
}

type creditGrantResponse struct {
	Id             string `json:"id"`
	UserId         string `json:"userId"`
	Name           string `json:"name"`
	AmountCents    int64  `json:"amountCents"`
	RemainingCents int64  `json:"remainingCents"`
	Currency       string `json:"currency"`
	Priority       int    `json:"priority"`
	Tags           string `json:"tags"`
}

type creditGrantListResponse struct {
	Grants []creditGrantResponse `json:"grants"`
	Count  int                   `json:"count"`
}

type creditBalanceItem struct {
	Currency  string `json:"currency"`
	Available int64  `json:"available"`
}

type creditBalanceResponse struct {
	UserId   string              `json:"userId"`
	Balances []creditBalanceItem `json:"balances"`
}

type pricingRuleResponse struct {
	Id        string `json:"id"`
	MeterId   string `json:"meterId"`
	PlanId    string `json:"planId"`
	Model     string `json:"model"`
	Currency  string `json:"currency"`
	UnitPrice int64  `json:"unitPrice"`
}

type pricingRuleListResponse struct {
	Rules []pricingRuleResponse `json:"rules"`
	Count int                   `json:"count"`
}

type invoiceLineItem struct {
	MeterId   string `json:"meterId"`
	MeterName string `json:"meterName"`
	Quantity  int64  `json:"quantity"`
	TotalCost int64  `json:"totalCost"`
}

type invoicePreviewResponse struct {
	UserId        string            `json:"userId"`
	LineItems     []invoiceLineItem `json:"lineItems"`
	Subtotal      int64             `json:"subtotal"`
	CreditApplied int64             `json:"creditApplied"`
	AmountDue     int64             `json:"amountDue"`
	Currency      string            `json:"currency"`
}

// Shared state across tests — populated in order by Ginkgo's sequential execution
var (
	meterId        string
	creditGrantId  string
	pricingRuleId  string
)

var _ = Describe("billing", func() {

	// ─── METERS ───────────────────────────────────────────────────────

	Context("Meters", func() {
		It("Should create a meter", func() {
			req := map[string]interface{}{
				"name":            "Input Tokens",
				"eventName":       "input_tokens",
				"aggregationType": "sum",
				"currency":        "usd",
				"dimensions":      []string{"model", "region"},
			}
			res := &meterResponse{}
			cl.Post("/billing/meters", req, res)

			Expect(res.Id).NotTo(BeEmpty())
			Expect(res.Name).To(Equal("Input Tokens"))
			Expect(res.EventName).To(Equal("input_tokens"))
			Expect(res.AggregationType).To(Equal("sum"))
			Expect(res.Currency).To(Equal("usd"))
			Expect(res.Dimensions).To(ConsistOf("model", "region"))

			meterId = res.Id
		})

		It("Should fail to create meter without name", func() {
			req := map[string]interface{}{
				"eventName": "missing_name",
			}
			res := &ApiError{}
			cl.Post("/billing/meters", req, res)

			Expect(res.Error.Message).To(ContainSubstring("name is required"))
		})

		It("Should fail to create meter without eventName", func() {
			req := map[string]interface{}{
				"name": "Missing Event",
			}
			res := &ApiError{}
			cl.Post("/billing/meters", req, res)

			Expect(res.Error.Message).To(ContainSubstring("eventName is required"))
		})

		It("Should list meters", func() {
			res := &meterListResponse{}
			cl.Get("/billing/meters", res)

			Expect(res.Count).To(BeNumerically(">=", 1))
			Expect(res.Meters).NotTo(BeEmpty())

			found := false
			for _, m := range res.Meters {
				if m.EventName == "input_tokens" {
					found = true
					Expect(m.Name).To(Equal("Input Tokens"))
				}
			}
			Expect(found).To(BeTrue())
		})

		It("Should get meter by ID", func() {
			res := &meterResponse{}
			cl.Get("/billing/meters/"+meterId, res)

			Expect(res.Id).To(Equal(meterId))
			Expect(res.Name).To(Equal("Input Tokens"))
			Expect(res.AggregationType).To(Equal("sum"))
		})

		It("Should return 404 for non-existent meter", func() {
			res := &ApiError{}
			cl.Get("/billing/meters/nonexistent-id", res)

			Expect(res.Error.Message).To(ContainSubstring("not found"))
		})
	})

	// ─── METER EVENTS ─────────────────────────────────────────────────

	Context("Meter Events", func() {
		It("Should record a single event", func() {
			req := map[string]interface{}{
				"events": []map[string]interface{}{
					{
						"meterId": meterId,
						"userId":  "hanzo/alice",
						"value":   int64(1500),
						"dimensions": map[string]interface{}{
							"model": "gpt-4",
						},
					},
				},
			}
			res := &eventsResponse{}
			cl.Post("/billing/meter-events", req, res)

			Expect(res.Count).To(Equal(1))
			Expect(res.Events[0].Status).To(Equal("created"))
			Expect(res.Events[0].Id).NotTo(BeEmpty())
		})

		It("Should record batch events", func() {
			req := map[string]interface{}{
				"events": []map[string]interface{}{
					{
						"meterId": meterId,
						"userId":  "hanzo/alice",
						"value":   int64(500),
					},
					{
						"meterId": meterId,
						"userId":  "hanzo/alice",
						"value":   int64(1000),
					},
				},
			}
			res := &eventsResponse{}
			cl.Post("/billing/meter-events", req, res)

			Expect(res.Count).To(Equal(2))
		})

		It("Should deduplicate events by idempotency key", func() {
			req := map[string]interface{}{
				"events": []map[string]interface{}{
					{
						"meterId":     meterId,
						"userId":      "hanzo/alice",
						"value":       int64(999),
						"idempotency": "dedup-key-1",
					},
				},
			}

			// First request
			res := &eventsResponse{}
			cl.Post("/billing/meter-events", req, res)
			Expect(res.Events[0].Status).To(Equal("created"))

			// Second request with same key
			res2 := &eventsResponse{}
			cl.Post("/billing/meter-events", req, res2)
			Expect(res2.Events[0].Status).To(Equal("duplicate"))
		})

		It("Should fail with empty events array", func() {
			req := map[string]interface{}{
				"events": []map[string]interface{}{},
			}
			res := &ApiError{}
			cl.Post("/billing/meter-events", req, res)

			Expect(res.Error.Message).To(ContainSubstring("at least one event"))
		})

		It("Should fail when meterId is missing", func() {
			req := map[string]interface{}{
				"events": []map[string]interface{}{
					{
						"userId": "hanzo/alice",
						"value":  int64(100),
					},
				},
			}
			res := &ApiError{}
			cl.Post("/billing/meter-events", req, res)

			Expect(res.Error.Message).To(ContainSubstring("meterId is required"))
		})

		It("Should fail when userId is missing", func() {
			req := map[string]interface{}{
				"events": []map[string]interface{}{
					{
						"meterId": meterId,
						"value":   int64(100),
					},
				},
			}
			res := &ApiError{}
			cl.Post("/billing/meter-events", req, res)

			Expect(res.Error.Message).To(ContainSubstring("userId is required"))
		})

		It("Should get aggregated summary (sum)", func() {
			res := &summaryResponse{}
			cl.Get("/billing/meter-events/summary?meterId="+meterId+"&userId=hanzo/alice", res)

			Expect(res.MeterId).To(Equal(meterId))
			Expect(res.MeterName).To(Equal("Input Tokens"))
			Expect(res.AggregationType).To(Equal("sum"))
			// 1500 + 500 + 1000 + 999 = 3999
			Expect(res.Value).To(BeNumerically(">=", 3999))
			Expect(res.EventCount).To(BeNumerically(">=", 4))
		})

		It("Should fail summary without meterId", func() {
			res := &ApiError{}
			cl.Get("/billing/meter-events/summary?userId=hanzo/alice", res)

			Expect(res.Error.Message).To(ContainSubstring("meterId"))
		})

		It("Should return 404 for summary with bad meterId", func() {
			res := &ApiError{}
			cl.Get("/billing/meter-events/summary?meterId=bad-id", res)

			Expect(res.Error.Message).To(ContainSubstring("not found"))
		})
	})

	// ─── CREDIT GRANTS ────────────────────────────────────────────────

	Context("Credit Grants", func() {
		It("Should create a credit grant", func() {
			req := map[string]interface{}{
				"userId":      "hanzo/alice",
				"name":        "Starter Credit",
				"amountCents": int64(500),
				"currency":    "usd",
				"priority":    0,
				"tags":        "starter",
				"expiresIn":   "720h",
			}
			res := &creditGrantResponse{}
			cl.Post("/billing/credit-grants", req, res)

			Expect(res.Id).NotTo(BeEmpty())
			Expect(res.UserId).To(Equal("hanzo/alice"))
			Expect(res.Name).To(Equal("Starter Credit"))
			Expect(res.AmountCents).To(Equal(int64(500)))
			Expect(res.RemainingCents).To(Equal(int64(500)))
			Expect(res.Currency).To(Equal("usd"))
			Expect(res.Priority).To(Equal(0))

			creditGrantId = res.Id
		})

		It("Should create a second grant with lower priority", func() {
			req := map[string]interface{}{
				"userId":      "hanzo/alice",
				"name":        "Promo $10",
				"amountCents": int64(1000),
				"currency":    "usd",
				"priority":    1,
				"tags":        "promo",
			}
			res := &creditGrantResponse{}
			cl.Post("/billing/credit-grants", req, res)

			Expect(res.Id).NotTo(BeEmpty())
			Expect(res.AmountCents).To(Equal(int64(1000)))
			Expect(res.Priority).To(Equal(1))
		})

		It("Should fail to create grant without userId", func() {
			req := map[string]interface{}{
				"name":        "Bad Grant",
				"amountCents": int64(100),
			}
			res := &ApiError{}
			cl.Post("/billing/credit-grants", req, res)

			Expect(res.Error.Message).To(ContainSubstring("userId is required"))
		})

		It("Should fail to create grant with zero amount", func() {
			req := map[string]interface{}{
				"userId":      "hanzo/alice",
				"amountCents": int64(0),
			}
			res := &ApiError{}
			cl.Post("/billing/credit-grants", req, res)

			Expect(res.Error.Message).To(ContainSubstring("amountCents must be positive"))
		})

		It("Should fail with invalid expiresIn duration", func() {
			req := map[string]interface{}{
				"userId":      "hanzo/alice",
				"name":        "Bad Expiry",
				"amountCents": int64(100),
				"expiresIn":   "not-a-duration",
			}
			res := &ApiError{}
			cl.Post("/billing/credit-grants", req, res)

			Expect(res.Error.Message).To(ContainSubstring("invalid expiresIn"))
		})

		It("Should list grants for a user", func() {
			res := &creditGrantListResponse{}
			cl.Get("/billing/credit-grants?userId=hanzo/alice", res)

			Expect(res.Count).To(BeNumerically(">=", 2))

			foundStarter := false
			foundPromo := false
			for _, g := range res.Grants {
				if g.Name == "Starter Credit" {
					foundStarter = true
				}
				if g.Name == "Promo $10" {
					foundPromo = true
				}
			}
			Expect(foundStarter).To(BeTrue())
			Expect(foundPromo).To(BeTrue())
		})

		It("Should fail list without userId", func() {
			res := &ApiError{}
			cl.Get("/billing/credit-grants", res)

			Expect(res.Error.Message).To(ContainSubstring("userId"))
		})

		It("Should get credit balance", func() {
			res := &creditBalanceResponse{}
			cl.Get("/billing/credit-balance?userId=hanzo/alice", res)

			Expect(res.UserId).To(Equal("hanzo/alice"))
			Expect(res.Balances).NotTo(BeEmpty())

			var usdBalance int64
			for _, b := range res.Balances {
				if b.Currency == "usd" {
					usdBalance = b.Available
				}
			}
			// 500 + 1000 = 1500
			Expect(usdBalance).To(Equal(int64(1500)))
		})

		It("Should void a credit grant", func() {
			res := &map[string]interface{}{}
			cl.Post("/billing/credit-grants/"+creditGrantId+"/void", nil, res)

			Expect((*res)["voided"]).To(Equal(true))
		})

		It("Should fail to void already voided grant", func() {
			res := &ApiError{}
			cl.Post("/billing/credit-grants/"+creditGrantId+"/void", nil, res)

			Expect(res.Error.Message).To(ContainSubstring("already voided"))
		})

		It("Should fail to void non-existent grant", func() {
			res := &ApiError{}
			cl.Post("/billing/credit-grants/nonexistent/void", nil, res)

			Expect(res.Error.Message).To(ContainSubstring("not found"))
		})

		It("Should reflect voided grant in balance", func() {
			res := &creditBalanceResponse{}
			cl.Get("/billing/credit-balance?userId=hanzo/alice", res)

			var usdBalance int64
			for _, b := range res.Balances {
				if b.Currency == "usd" {
					usdBalance = b.Available
				}
			}
			// Only promo remains: 1000
			Expect(usdBalance).To(Equal(int64(1000)))
		})
	})

	// ─── PRICING RULES ───────────────────────────────────────────────

	Context("Pricing Rules", func() {
		It("Should create a per-unit pricing rule", func() {
			req := map[string]interface{}{
				"meterId":   meterId,
				"model":     "per_unit",
				"currency":  "usd",
				"unitPrice": int64(1), // 1 cent per unit
			}
			res := &pricingRuleResponse{}
			cl.Post("/billing/pricing-rules", req, res)

			Expect(res.Id).NotTo(BeEmpty())
			Expect(res.MeterId).To(Equal(meterId))
			Expect(res.BaseModel).To(Equal("per_unit"))
			Expect(res.UnitPrice).To(Equal(int64(1)))

			pricingRuleId = res.Id
		})

		It("Should fail to create rule without meterId", func() {
			req := map[string]interface{}{
				"model":     "per_unit",
				"unitPrice": int64(1),
			}
			res := &ApiError{}
			cl.Post("/billing/pricing-rules", req, res)

			Expect(res.Error.Message).To(ContainSubstring("meterId is required"))
		})

		It("Should list pricing rules", func() {
			res := &pricingRuleListResponse{}
			cl.Get("/billing/pricing-rules", res)

			Expect(res.Count).To(BeNumerically(">=", 1))
		})

		It("Should filter pricing rules by meterId", func() {
			res := &pricingRuleListResponse{}
			cl.Get("/billing/pricing-rules?meterId="+meterId, res)

			Expect(res.Count).To(BeNumerically(">=", 1))
			for _, r := range res.Rules {
				Expect(r.MeterId).To(Equal(meterId))
			}
		})

		It("Should delete a pricing rule", func() {
			// Create a throwaway rule to delete
			createReq := map[string]interface{}{
				"meterId":   meterId,
				"model":     "per_unit",
				"currency":  "usd",
				"unitPrice": int64(99),
			}
			createRes := &pricingRuleResponse{}
			cl.Post("/billing/pricing-rules", createReq, createRes)

			Expect(createRes.Id).NotTo(BeEmpty())

			// Delete it
			deleteRes := &map[string]interface{}{}
			cl.Delete("/billing/pricing-rules/" + createRes.Id)

			_ = deleteRes
		})

		It("Should fail to delete non-existent rule", func() {
			res := &ApiError{}
			cl.Delete("/billing/pricing-rules/nonexistent-id", res)

			Expect(res.Error.Message).To(ContainSubstring("not found"))
		})
	})

	// ─── INVOICE PREVIEW ──────────────────────────────────────────────

	Context("Invoice Preview", func() {
		It("Should calculate invoice preview with usage and credits", func() {
			req := map[string]interface{}{
				"userId":      "hanzo/alice",
				"periodStart": "2020-01-01",
				"periodEnd":   "2099-12-31",
			}
			res := &invoicePreviewResponse{}
			cl.Post("/billing/invoice-preview", req, res)

			Expect(res.UserId).To(Equal("hanzo/alice"))
			Expect(res.Currency).To(Equal("usd"))

			// Should have line items from meter events
			Expect(len(res.LineItems)).To(BeNumerically(">=", 1))

			// Subtotal should be usage * pricing
			Expect(res.Subtotal).To(BeNumerically(">", 0))

			// Credits should be applied
			Expect(res.CreditApplied).To(BeNumerically(">=", 0))

			// Amount due should be subtotal - credits (but >= 0)
			Expect(res.AmountDue).To(BeNumerically(">=", 0))
			Expect(res.Subtotal).To(Equal(res.CreditApplied + res.AmountDue))
		})

		It("Should fail without userId", func() {
			req := map[string]interface{}{
				"periodStart": "2026-02-01",
				"periodEnd":   "2026-03-01",
			}
			res := &ApiError{}
			cl.Post("/billing/invoice-preview", req, res)

			Expect(res.Error.Message).To(ContainSubstring("userId is required"))
		})

		It("Should return empty preview for user with no usage", func() {
			req := map[string]interface{}{
				"userId":      "hanzo/nobody",
				"periodStart": "2026-01-01",
				"periodEnd":   "2026-02-01",
			}
			res := &invoicePreviewResponse{}
			cl.Post("/billing/invoice-preview", req, res)

			Expect(res.Subtotal).To(Equal(int64(0)))
			Expect(res.CreditApplied).To(Equal(int64(0)))
			Expect(res.AmountDue).To(Equal(int64(0)))
		})
	})

	// ─── EXISTING ENDPOINTS (balance, usage, deposit, refund) ─────────

	Context("Existing Billing Endpoints", func() {
		It("Should record usage via POST /billing/usage", func() {
			req := map[string]interface{}{
				"user":             "hanzo/bob",
				"currency":         "usd",
				"amount":           int64(25),
				"model":            "gpt-4",
				"provider":         "openai",
				"promptTokens":     100,
				"completionTokens": 50,
				"totalTokens":      150,
				"requestId":        "req-test-1",
			}
			res := &map[string]interface{}{}
			cl.Post("/billing/usage", req, res)

			Expect((*res)["transactionId"]).NotTo(BeEmpty())
			Expect((*res)["type"]).To(Equal("withdraw"))
		})

		It("Should skip zero-cost usage", func() {
			req := map[string]interface{}{
				"user":   "hanzo/bob",
				"amount": int64(0),
			}
			res := &map[string]interface{}{}
			cl.Post("/billing/usage", req, res)

			Expect((*res)["status"]).To(Equal("skipped"))
		})

		It("Should create a deposit", func() {
			req := map[string]interface{}{
				"user":     "hanzo/bob",
				"currency": "usd",
				"amount":   int64(1000),
				"notes":    "Test deposit",
				"tags":     "test",
			}
			res := &map[string]interface{}{}
			cl.Post("/billing/deposit", req, res)

			Expect((*res)["transactionId"]).NotTo(BeEmpty())
			Expect((*res)["type"]).To(Equal("deposit"))
		})

		It("Should grant starter credit", func() {
			req := map[string]interface{}{
				"user": "hanzo/charlie",
			}
			res := &map[string]interface{}{}
			cl.Post("/billing/credit", req, res)

			Expect((*res)["transactionId"]).NotTo(BeEmpty())
			Expect((*res)["tags"]).To(Equal("starter-credit"))
		})

		It("Should create a refund", func() {
			// First create a deposit
			depositReq := map[string]interface{}{
				"user":     "hanzo/dave",
				"currency": "usd",
				"amount":   int64(200),
			}
			depositRes := &map[string]interface{}{}
			cl.Post("/billing/deposit", depositReq, depositRes)

			txId := (*depositRes)["transactionId"].(string)

			// Then refund it
			refundReq := map[string]interface{}{
				"user":                  "hanzo/dave",
				"currency":              "usd",
				"amount":                int64(100),
				"originalTransactionId": txId,
				"notes":                 "Partial refund",
			}
			refundRes := &map[string]interface{}{}
			cl.Post("/billing/refund", refundReq, refundRes)

			Expect((*refundRes)["transactionId"]).NotTo(BeEmpty())
		})
	})
})
