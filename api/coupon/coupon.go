package coupon

import (
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/coupon"
	"github.com/hanzoai/commerce/models/creditgrant"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/rest"
)

func getCoupon(c *zip.Ctx) error {
	couponid := c.Param("couponid")

	db := datastore.NewNamespaced(c.Context())
	cpn := coupon.New(db)

	if err := cpn.GetById(couponid); err != nil {
		return http.Fail(c, 404, "Failed to get coupon", err)
	}

	// if cpn.Dynamic {
	// 	http.Fail(c, 400, "Failed to get dynamic coupon", nil)
	// 	return
	// }

	// Check if coupon has been used
	cpn.Enabled = cpn.Redeemable()

	return http.Render(c, 200, cpn)
}

func codeFromId(c *zip.Ctx) error {
	couponid := c.Param("couponid")
	uniqueid := c.Param("uniqueid")

	db := datastore.NewNamespaced(c.Context())
	cpn := coupon.New(db)
	if err := cpn.GetById(couponid); err != nil {
		return http.Fail(c, 404, "Failed to get coupon", err)
	}

	cpn.Code_ = cpn.CodeFromId(uniqueid)

	log.Debug("%#v", cpn)

	// Check if coupon has been used
	cpn.Enabled = cpn.Redeemable()

	return http.Render(c, 200, cpn)
}

func codeFromList(c *zip.Ctx) error {
	couponid := c.Param("couponid")

	db := datastore.NewNamespaced(c.Context())
	cpn := coupon.New(db)
	if err := cpn.GetById(couponid); err != nil {
		return http.Fail(c, 404, "Failed to get coupon %v", err)
	}

	list := make([]string, 0)

	// Decode response body to create new order
	if err := json.DecodeBytes(c.Body(), list); err != nil {
		return http.Fail(c, 400, "Failed decode request body", err)
	}

	codes := make([]string, len(list))

	for _, id := range list {
		codes = append(codes, cpn.CodeFromId(id))
	}

	return http.Render(c, 200, codes)
}

// couponReward describes a reward granted by coupon redemption.
type couponReward struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Value       int    `json:"value"`
	Duration    string `json:"duration,omitempty"`
	RedeemURL   string `json:"redeemUrl"`
}

// tryfreeRewards returns the static reward set for the TRYFREE coupon.
func tryfreeRewards() []couponReward {
	return []couponReward{
		{
			Type:        "bot-trial",
			Description: "$5/mo Hanzo Bot trial",
			Value:       500,
			Duration:    "1 month",
			RedeemURL:   "https://hanzo.bot",
		},
		{
			Type:        "compute-credits",
			Description: "$5 compute credits on Hanzo Console",
			Value:       500,
			RedeemURL:   "https://console.hanzo.ai",
		},
	}
}

func validateCoupon(c *zip.Ctx) error {
	var req struct {
		Code string `json:"code"`
	}
	if err := json.DecodeBytes(c.Body(), &req); err != nil {
		return http.Fail(c, 400, "Invalid request body", err)
	}

	db := datastore.New(c.Context())
	cpn := coupon.New(db)
	if ok, err := cpn.Query().Filter("Code_=", req.Code).Get(); err != nil || !ok {
		return http.Render(c, 200, map[string]any{"valid": false, "error": "Coupon not found"})
	}

	if !cpn.Redeemable() {
		return http.Render(c, 200, map[string]any{"valid": false, "error": "Coupon expired or fully redeemed"})
	}

	// This endpoint is PUBLIC (pre-sign-up promo check), so expose ONLY the
	// fields a checkout/pricing page needs to display and apply the discount.
	// Internal accounting/attribution — used, limit, campaignId, referrerId,
	// dynamic, enabled, start/end dates — is never leaked to anonymous callers.
	result := map[string]any{
		"valid": true,
		"coupon": map[string]any{
			"name":          cpn.Name,
			"type":          cpn.Type,
			"code":          cpn.Code_,
			"amount":        cpn.Amount,
			"filter":        cpn.Filter,
			"once":          cpn.Once,
			"productId":     cpn.ProductId,
			"freeProductId": cpn.FreeProductId,
			"freeVariantId": cpn.FreeVariantId,
			"freeQuantity":  cpn.FreeQuantity,
		},
	}

	if cpn.Code_ == "TRYFREE" {
		result["rewards"] = tryfreeRewards()
	}

	return http.Render(c, 200, result)
}

func redeemCoupon(c *zip.Ctx) error {
	var req struct {
		Code string `json:"code"`
	}
	if err := json.DecodeBytes(c.Body(), &req); err != nil {
		return http.Fail(c, 400, "Invalid request body", err)
	}

	// ┌─ DO NOT "FIX" THIS TO datastore.NewNamespaced ─────────────────────────┐
	// This lookup runs in the ROOT namespace while coupon CRUD (rest.New below in
	// Route) writes ORG-namespaced rows. So an org's own coupon is invisible here
	// and redeem 404s. That asymmetry looks exactly like a namespacing bug, and
	// making it "consistent" is a one-word change that opens a self-mint:
	//
	//	coupon CRUD is org-admin reachable → an org admin POSTs a coupon with
	//	Amount = 100000000 → redeem finds it → the generic branch below mints
	//	creditgrant.AmountCents = cpn.Amount → and credit grants ARE spendable
	//	(getActiveGrants → BurnCredits) → unlimited free inference.
	//
	// The namespace mismatch is the ONLY thing preventing that today: this route
	// fails closed BY ACCIDENT, not by control. Before touching it, put the amount
	// under platform authority (mint-gate coupon CRUD, or derive the reward
	// server-side from a fixed table the way api/referral derives from
	// tier.Rewards). See the mint surface guard: api/billing/mint_surface_test.go.
	// └────────────────────────────────────────────────────────────────────────┘
	db := datastore.New(c.Context())
	cpn := coupon.New(db)
	if ok, err := cpn.Query().Filter("Code_=", req.Code).Get(); err != nil || !ok {
		return http.Fail(c, 404, "Coupon not found", nil)
	}

	if !cpn.Redeemable() {
		return http.Fail(c, 400, "Coupon expired or fully redeemed", nil)
	}

	// Get authenticated user ID
	userId := c.Locals("userId")
	uid, _ := userId.(string)
	if uid == "" {
		return http.Fail(c, 401, "Authentication required", nil)
	}

	// Check if user already redeemed this coupon (via credit grant tag)
	existing := make([]creditgrant.CreditGrant, 0)
	if _, err := creditgrant.Query(db).Filter("UserId=", uid).Filter("Tags=", "coupon:"+cpn.Code_).GetAll(&existing); err == nil && len(existing) > 0 {
		return http.Fail(c, 400, "Coupon already redeemed", nil)
	}

	rewards := make([]couponReward, 0)
	now := time.Now()

	if cpn.Code_ == "TRYFREE" {
		// Grant 1: $5 bot trial credit (eligible for bot-execution meter)
		botGrant := creditgrant.New(db)
		botGrant.UserId = uid
		botGrant.Name = "TRYFREE - Hanzo Bot Trial"
		botGrant.AmountCents = 500
		botGrant.RemainingCents = 500
		botGrant.Currency = "usd"
		botGrant.EffectiveAt = now
		botGrant.ExpiresAt = now.AddDate(0, 1, 0) // 1 month
		botGrant.Priority = 0
		botGrant.Eligibility = []string{"bot-execution"}
		botGrant.Tags = "promo,coupon:" + cpn.Code_
		botGrant.MustPut()

		// Grant 2: $5 compute credits (eligible for all compute meters)
		computeGrant := creditgrant.New(db)
		computeGrant.UserId = uid
		computeGrant.Name = "TRYFREE - Compute Credits"
		computeGrant.AmountCents = 500
		computeGrant.RemainingCents = 500
		computeGrant.Currency = "usd"
		computeGrant.EffectiveAt = now
		computeGrant.ExpiresAt = now.AddDate(0, 3, 0) // 3 months
		computeGrant.Priority = 0
		computeGrant.Eligibility = []string{"api-usage", "inference"}
		computeGrant.Tags = "promo,coupon:" + cpn.Code_
		computeGrant.MustPut()

		rewards = tryfreeRewards()
	} else {
		// Generic coupon: grant flat credit
		grant := creditgrant.New(db)
		grant.UserId = uid
		grant.Name = cpn.Name
		grant.AmountCents = int64(cpn.Amount)
		grant.RemainingCents = int64(cpn.Amount)
		grant.Currency = "usd"
		grant.EffectiveAt = now
		grant.ExpiresAt = now.AddDate(0, 1, 0)
		grant.Priority = 1
		grant.Tags = "promo,coupon:" + cpn.Code_
		grant.MustPut()

		rewards = append(rewards, couponReward{
			Type:        "credit",
			Description: cpn.Name,
			Value:       cpn.Amount,
		})
	}

	// Increment coupon usage
	cpn.Used++
	cpn.MustPut()

	log.Info("Coupon %s redeemed by user %s", cpn.Code_, uid)

	return http.Render(c, 200, map[string]any{
		"success": true,
		"rewards": rewards,
	})
}

func Route(router zip.Router, args ...zip.Handler) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	tokenRequired := middleware.TokenRequired(permission.User)
	namespaced := middleware.Namespace()

	api := rest.New(coupon.Coupon{})

	api.Get = getCoupon
	api.GET("/:couponid/code/:uniqueid", adminRequired, namespaced, codeFromId)
	api.POST("/:couponid/code", adminRequired, namespaced, codeFromList)
	// Public: the marketing pricing page (commerce.hanzo.ai/commerce) validates
	// promo codes before sign-up, so there is no user token yet. Validation is
	// read-only and never grants anything — /redeem below stays token-gated.
	api.POST("/validate", validateCoupon)
	api.POST("/redeem", tokenRequired, redeemCoupon)

	api.Route(router, args...)
}
