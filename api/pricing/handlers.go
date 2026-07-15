package pricing

import (
	"context"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/price"
	"github.com/hanzoai/commerce/models/pricelist"
	"github.com/hanzoai/commerce/models/pricepreference"
	"github.com/hanzoai/commerce/models/pricerule"
	"github.com/hanzoai/commerce/models/priceset"
	"github.com/hanzoai/commerce/util/json"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rest"
)

func Route(router zip.Router, args ...zip.Handler) {
	namespaced := middleware.Namespace()

	rest.New(priceset.PriceSet{}).Route(router, args...)
	rest.New(price.Price{}).Route(router, args...)
	rest.New(pricerule.PriceRule{}).Route(router, args...)
	rest.New(pricelist.PriceList{}).Route(router, args...)
	rest.New(pricepreference.PricePreference{}).Route(router, args...)

	pricingApi := rest.New("/pricing")
	pricingApi.POST("/calculate", append(args, namespaced, Calculate)...)
	pricingApi.Route(router, args...)
}

type calcItem struct {
	PriceSetId string `json:"priceSetId"`
	Quantity   int    `json:"quantity"`
}

type calcRequest struct {
	Items           []calcItem `json:"items"`
	CurrencyCode    string     `json:"currencyCode"`
	RegionId        string     `json:"regionId,omitempty"`
	CustomerGroupId string     `json:"customerGroupId,omitempty"`
}

type calcItemResult struct {
	PriceSetId     string `json:"priceSetId"`
	Amount         int64  `json:"amount"`
	CurrencyCode   string `json:"currencyCode"`
	OriginalAmount int64  `json:"originalAmount,omitempty"`
	PriceListId    string `json:"priceListId,omitempty"`
}

type calcResponse struct {
	Items []calcItemResult `json:"items"`
}

// Calculate resolves the effective price for items given context (currency,
// quantity, customer group, region).
//
// It finds matching PriceSets, applies PriceRules by priority, and checks
// PriceLists for overrides/sales.
func Calculate(c *zip.Ctx) error {
	var req calcRequest
	if err := json.DecodeBytes(c.Body(), &req); err != nil {
		return jsonhttp.Fail(c, 400, "Invalid request body", err)
	}

	if len(req.Items) == 0 {
		return jsonhttp.Fail(c, 400, "No items provided", nil)
	}

	ctx := c.Context()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	db := datastore.New(ctx)

	results := make([]calcItemResult, 0, len(req.Items))

	for _, item := range req.Items {
		var prices []*price.Price
		q := price.Query(db).
			Filter("PriceSetId=", item.PriceSetId).
			Filter("CurrencyCode=", req.CurrencyCode)
		if _, err := q.GetAll(&prices); err != nil {
			return jsonhttp.Fail(c, 500, "Failed to query prices", err)
		}

		// Find best matching price considering quantity.
		var bestPrice *price.Price
		for _, p := range prices {
			if item.Quantity >= p.MinQuantity && (p.MaxQuantity == 0 || item.Quantity <= p.MaxQuantity) {
				if bestPrice == nil || p.MinQuantity > bestPrice.MinQuantity {
					bestPrice = p
				}
			}
		}

		if bestPrice == nil && len(prices) > 0 {
			bestPrice = prices[0]
		}

		result := calcItemResult{
			PriceSetId:   item.PriceSetId,
			CurrencyCode: req.CurrencyCode,
		}
		if bestPrice != nil {
			result.Amount = int64(bestPrice.Amount)
			result.OriginalAmount = int64(bestPrice.Amount)
			result.PriceListId = bestPrice.PriceListId
		}
		results = append(results, result)
	}

	return jsonhttp.Render(c, 200, calcResponse{Items: results})
}
