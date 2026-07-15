package promotion

import (
	"context"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/applicationmethod"
	"github.com/hanzoai/commerce/models/campaignbudget"
	promotionModel "github.com/hanzoai/commerce/models/promotion"
	"github.com/hanzoai/commerce/models/promotionrule"
	"github.com/hanzoai/commerce/util/json"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rest"
)

func Route(router zip.Router, args ...zip.Handler) {
	namespaced := middleware.Namespace()

	rest.New(promotionModel.Promotion{}).Route(router, args...)
	rest.New(applicationmethod.ApplicationMethod{}).Route(router, args...)
	rest.New(promotionrule.PromotionRule{}).Route(router, args...)
	rest.New(campaignbudget.CampaignBudget{}).Route(router, args...)

	promoApi := rest.New("/promotion")
	promoApi.POST("/evaluate", append(args, namespaced, Evaluate)...)
	promoApi.Route(router, args...)
}

type evalItem struct {
	ProductId string `json:"productId"`
	VariantId string `json:"variantId"`
	Quantity  int    `json:"quantity"`
	Amount    int64  `json:"amount"`
}

type evalRequest struct {
	Items           []evalItem `json:"items"`
	CurrencyCode    string     `json:"currencyCode"`
	RegionId        string     `json:"regionId,omitempty"`
	CustomerGroupId string     `json:"customerGroupId,omitempty"`
	CartTotal       int64      `json:"cartTotal"`
}

type adjustment struct {
	PromotionId string `json:"promotionId"`
	Code        string `json:"code"`
	Amount      int64  `json:"amount"`
	Type        string `json:"type"`
}

type evalResponse struct {
	Adjustments   []adjustment `json:"adjustments"`
	TotalDiscount int64        `json:"totalDiscount"`
}

// Evaluate finds applicable promotions for a given cart/order context.
//
// It queries all active automatic promotions, checks date ranges and
// currency constraints, then calculates discount adjustments using the
// associated ApplicationMethod.
func Evaluate(c *zip.Ctx) error {
	var req evalRequest
	if err := json.DecodeBytes(c.Body(), &req); err != nil {
		return jsonhttp.Fail(c, 400, "Invalid request body", err)
	}

	ctx := c.Context()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	db := datastore.New(ctx)

	now := time.Now()
	var promotions []*promotionModel.Promotion
	q := promotionModel.Query(db).Filter("Status=", "active").Filter("IsAutomatic=", true)
	if _, err := q.GetAll(&promotions); err != nil {
		return jsonhttp.Fail(c, 500, "Failed to query promotions", err)
	}

	adjustments := make([]adjustment, 0)
	totalDiscount := int64(0)

	for _, promo := range promotions {
		if promo.StartsAt != nil && promo.StartsAt.After(now) {
			continue
		}
		if promo.EndsAt != nil && promo.EndsAt.Before(now) {
			continue
		}

		am := applicationmethod.New(db)
		ok, err := am.Query().Filter("PromotionId=", promo.Id()).Get()
		if err != nil || !ok {
			continue
		}

		if am.CurrencyCode != "" && am.CurrencyCode != req.CurrencyCode {
			continue
		}

		var discountAmount int64
		switch am.Type {
		case "percentage":
			if am.TargetType == "order" {
				// Value is in basis points: 1500 = 15.00%
				discountAmount = req.CartTotal * int64(am.Value) / 10000
			}
		case "fixed":
			discountAmount = int64(am.Value)
		}

		if discountAmount > 0 {
			adjustments = append(adjustments, adjustment{
				PromotionId: promo.Id(),
				Code:        promo.Code,
				Amount:      discountAmount,
				Type:        am.Type,
			})
			totalDiscount += discountAmount
		}
	}

	return jsonhttp.Render(c, 200, evalResponse{
		Adjustments:   adjustments,
		TotalDiscount: totalDiscount,
	})
}
