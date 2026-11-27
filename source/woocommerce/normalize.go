// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package woocommerce

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/source/hub"
	"github.com/hanzoai/commerce/source/shopify"
)

const (
	MetaConsentKey = "_wc_other/hanzo/marketing"
	MetaConsentAt  = "hanzo_consent_at"
)

type wcMeta struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

func parseConsentMeta(meta []wcMeta) (state string, at string) {
	state = "unknown"
	for _, m := range meta {
		val := strings.TrimSpace(stringifyAny(m.Value))
		switch m.Key {
		case MetaConsentKey:
			switch val {
			case "1":
				state = "subscribed"
			case "0":
				state = "unsubscribed"
			}
		case MetaConsentAt:
			at = val
		}
	}
	return state, at
}

func stringifyAny(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatInt(int64(x), 10)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case bool:
		if x {
			return "1"
		}
		return "0"
	default:
		return ""
	}
}

func slugFromPermalink(permalink, fallback string) string {
	if permalink != "" {
		if u, err := url.Parse(permalink); err == nil {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) > 0 && parts[len(parts)-1] != "" {
				return parts[len(parts)-1]
			}
		}
	}
	return fallback
}

func decodeArrayOrSingle[T any](raw []byte) ([]T, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	if trimmed[0] == '[' {
		var rows []T
		if err := dec.Decode(&rows); err != nil {
			return nil, err
		}
		return rows, nil
	}
	var one T
	if err := dec.Decode(&one); err != nil {
		return nil, err
	}
	return []T{one}, nil
}

type wcProductRow struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	Status          string `json:"status"`
	Permalink       string `json:"permalink"`
	SKU             string `json:"sku"`
	GlobalUniqueID  string `json:"global_unique_id"`
	Price           string `json:"price"`
	RegularPrice    string `json:"regular_price"`
	SalePrice       string `json:"sale_price"`
	ManageStock     bool   `json:"manage_stock"`
	StockQuantity   *int   `json:"stock_quantity"`
	StockStatus     string `json:"stock_status"`
	DateCreatedGMT  string `json:"date_created_gmt"`
	DateModifiedGMT string `json:"date_modified_gmt"`
	Currency        string `json:"currency"`
	Images          []struct {
		ID  int    `json:"id"`
		Src string `json:"src"`
	} `json:"images"`
	Variations []struct {
		ID              int    `json:"id"`
		Permalink       string `json:"permalink"`
		SKU             string `json:"sku"`
		Price           string `json:"price"`
		RegularPrice    string `json:"regular_price"`
		SalePrice       string `json:"sale_price"`
		Status          string `json:"status"`
		ManageStock     bool   `json:"manage_stock"`
		StockQuantity   *int   `json:"stock_quantity"`
		StockStatus     string `json:"stock_status"`
		DateModifiedGMT string `json:"date_modified_gmt"`
		Image           *struct {
			ID  int    `json:"id"`
			Src string `json:"src"`
		} `json:"image"`
		Attributes []struct {
			Name   string `json:"name"`
			Option string `json:"option"`
		} `json:"attributes"`
	} `json:"variations"`
}

// NormalizeProducts parses a WooCommerce plugin batch (matching tests/golden/product.json)
// or single REST v3 product into hub.CanonicalProduct records.
func NormalizeProducts(raw []byte, defaultCurrency string) ([]hub.CanonicalProduct, error) {
	rows, err := decodeArrayOrSingle[wcProductRow](raw)
	if err != nil {
		return nil, err
	}
	out := make([]hub.CanonicalProduct, 0, len(rows))
	for _, r := range rows {
		cur := r.Currency
		if cur == "" {
			cur = defaultCurrency
		}
		if cur == "" {
			cur = "USD"
		}
		published := strings.EqualFold(r.Status, "publish")
		inStock := !strings.EqualFold(r.StockStatus, "outofstock")
		pPrice := shopify.ParseMoneyCents(r.Price, cur)
		if pPrice == 0 && r.RegularPrice != "" {
			pPrice = shopify.ParseMoneyCents(r.RegularPrice, cur)
		}
		pInv := 0
		if r.ManageStock && r.StockQuantity != nil {
			pInv = *r.StockQuantity
		}
		imgs := make([]string, 0, len(r.Images))
		for _, im := range r.Images {
			if im.Src != "" {
				imgs = append(imgs, im.Src)
			}
		}

		vars := make([]hub.CanonicalVariant, 0, len(r.Variations))
		anyVarManaged := false
		varStockSum := 0
		for _, v := range r.Variations {
			vPrice := shopify.ParseMoneyCents(v.Price, cur)
			if vPrice == 0 && v.RegularPrice != "" {
				vPrice = shopify.ParseMoneyCents(v.RegularPrice, cur)
			}
			vMSRP := shopify.ParseMoneyCents(v.RegularPrice, cur)
			vInv := 0
			if v.ManageStock && v.StockQuantity != nil {
				vInv = *v.StockQuantity
				anyVarManaged = true
				varStockSum += vInv
			}
			vPub := v.Status == "" || strings.EqualFold(v.Status, "publish")
			vInStock := !strings.EqualFold(v.StockStatus, "outofstock") && (!v.ManageStock || vInv > 0)
			opts := map[string]string{}
			optLabels := make([]string, 0, len(v.Attributes))
			for _, a := range v.Attributes {
				opts[a.Name] = a.Option
				if a.Option != "" {
					optLabels = append(optLabels, a.Option)
				}
			}
			vName := r.Name
			if len(optLabels) > 0 {
				vName = r.Name + " - " + strings.Join(optLabels, " / ")
			}
			imgURL := ""
			if v.Image != nil {
				imgURL = v.Image.Src
			}
			vars = append(vars, hub.CanonicalVariant{
				ExternalId:  strconv.Itoa(v.ID),
				SKU:         strings.TrimSpace(v.SKU),
				Name:        vName,
				PriceCents:  vPrice,
				MSRPCents:   vMSRP,
				Available:   published && vPub && vInStock,
				ManageStock: v.ManageStock,
				Inventory:   vInv,
				ImageURL:    imgURL,
				Options:     opts,
			})
		}

		manageParent := r.ManageStock
		totalInv := pInv
		if !manageParent && anyVarManaged {
			manageParent = true
			totalInv = varStockSum
		}

		out = append(out, hub.CanonicalProduct{
			ExternalId:  strconv.Itoa(r.ID),
			Slug:        slugFromPermalink(r.Permalink, strings.ToLower(strings.TrimSpace(r.SKU))),
			SKU:         strings.TrimSpace(r.SKU),
			UPC:         strings.TrimSpace(r.GlobalUniqueID),
			Name:        strings.TrimSpace(r.Name),
			Currency:    strings.ToUpper(cur),
			PriceCents:  pPrice,
			Available:   published && inStock,
			Hidden:      !published,
			ManageStock: manageParent,
			Inventory:   totalInv,
			Images:      imgs,
			Variants:    vars,
		})
	}
	return out, nil
}

type wcCustomerRow struct {
	ID              int      `json:"id"`
	Email           string   `json:"email"`
	FirstName       string   `json:"first_name"`
	LastName        string   `json:"last_name"`
	Role            string   `json:"role"`
	DateCreatedGMT  string   `json:"date_created_gmt"`
	DateModifiedGMT string   `json:"date_modified_gmt"`
	MetaData        []wcMeta `json:"meta_data"`
}

// NormalizeCustomers parses WooCommerce REST v3 customers (matching tests/golden/customer.json).
func NormalizeCustomers(raw []byte) ([]hub.CanonicalCustomer, error) {
	rows, err := decodeArrayOrSingle[wcCustomerRow](raw)
	if err != nil {
		return nil, err
	}
	out := make([]hub.CanonicalCustomer, 0, len(rows))
	for _, r := range rows {
		consent, at := parseConsentMeta(r.MetaData)
		out = append(out, hub.CanonicalCustomer{
			ExternalId:       strconv.Itoa(r.ID),
			Email:            strings.ToLower(strings.TrimSpace(r.Email)),
			FirstName:        strings.TrimSpace(r.FirstName),
			LastName:         strings.TrimSpace(r.LastName),
			MarketingConsent: consent,
			ConsentUpdatedAt: at,
		})
	}
	return out, nil
}

type wcOrderRow struct {
	ID             int    `json:"id"`
	Number         string `json:"number"`
	Status         string `json:"status"`
	Currency       string `json:"currency"`
	Total          string `json:"total"`
	DiscountTotal  string `json:"discount_total"`
	CustomerID     int    `json:"customer_id"`
	Billing        struct {
		Email string `json:"email"`
	} `json:"billing"`
	DateCreatedGMT  string `json:"date_created_gmt"`
	DateModifiedGMT string `json:"date_modified_gmt"`
	DatePaidGMT     string `json:"date_paid_gmt"`
	LineItems       []struct {
		ID          int    `json:"id"`
		ProductID   int    `json:"product_id"`
		VariationID int    `json:"variation_id"`
		Quantity    int    `json:"quantity"`
		SKU         string `json:"sku"`
		Name        string `json:"name"`
		Total       string `json:"total"`
		Price       string `json:"price"`
	} `json:"line_items"`
	CouponLines []struct {
		Code string `json:"code"`
	} `json:"coupon_lines"`
	Refunds []struct {
		ID    int    `json:"id"`
		Total string `json:"total"`
	} `json:"refunds"`
	MetaData []wcMeta `json:"meta_data"`
}

// NormalizeOrders parses WooCommerce REST v3 orders (matching tests/golden/order.json).
func NormalizeOrders(raw []byte, defaultCurrency string) ([]hub.CanonicalOrder, error) {
	rows, err := decodeArrayOrSingle[wcOrderRow](raw)
	if err != nil {
		return nil, err
	}
	out := make([]hub.CanonicalOrder, 0, len(rows))
	for _, r := range rows {
		cur := r.Currency
		if cur == "" {
			cur = defaultCurrency
		}
		if cur == "" {
			cur = "USD"
		}
		st := strings.ToLower(strings.TrimSpace(r.Status))
		paid := strings.TrimSpace(r.DatePaidGMT) != "" || st == "processing" || st == "completed"
		normStatus := "open"
		switch st {
		case "completed":
			normStatus = "completed"
		case "cancelled", "canceled", "failed", "refunded", "trash":
			normStatus = "cancelled"
		case "on-hold":
			normStatus = "on-hold"
		}

		lines := make([]hub.CanonicalOrderLine, 0, len(r.LineItems))
		for _, li := range r.LineItems {
			extVar := ""
			if li.VariationID > 0 {
				extVar = strconv.Itoa(li.VariationID)
			}
			lines = append(lines, hub.CanonicalOrderLine{
				ExternalId:        strconv.Itoa(li.ID),
				ExternalProductId: strconv.Itoa(li.ProductID),
				ExternalVariantId: extVar,
				SKU:               strings.TrimSpace(li.SKU),
				Name:              li.Name,
				Quantity:          li.Quantity,
				UnitPriceCents:    shopify.ParseMoneyCents(li.Price, cur),
				TotalCents:        shopify.ParseMoneyCents(li.Total, cur),
			})
		}

		codes := make([]string, 0, len(r.CouponLines))
		for _, cl := range r.CouponLines {
			if cl.Code != "" {
				codes = append(codes, cl.Code)
			}
		}

		var refunded currency.Cents
		for _, rf := range r.Refunds {
			amt := shopify.ParseMoneyCents(rf.Total, cur)
			if amt < 0 {
				amt = -amt
			}
			refunded += amt
		}

		consent, consentAt := parseConsentMeta(r.MetaData)
		custExt := ""
		if r.CustomerID > 0 {
			custExt = strconv.Itoa(r.CustomerID)
		}

		out = append(out, hub.CanonicalOrder{
			ExternalId:       strconv.Itoa(r.ID),
			Number:           r.Number,
			Status:           normStatus,
			Paid:             paid,
			Currency:         strings.ToUpper(cur),
			TotalCents:       shopify.ParseMoneyCents(r.Total, cur),
			DiscountCents:    shopify.ParseMoneyCents(r.DiscountTotal, cur),
			RefundedCents:    refunded,
			CustomerExternal: custExt,
			Email:            strings.ToLower(strings.TrimSpace(r.Billing.Email)),
			CouponCodes:      codes,
			Lines:            lines,
			MarketingConsent: consent,
			ConsentUpdatedAt: consentAt,
		})
	}
	return out, nil
}
