// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package shopify

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/source/hub"
)

// CurrencyDecimals returns the minor-unit exponent for ISO-4217 currency codes.
func CurrencyDecimals(code string) int {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "BIF", "CLP", "DJF", "GNF", "ISK", "JPY", "KMF", "KRW", "MGA", "PYG", "RWF", "UGX", "VND", "VUV", "XAF", "XOF", "XPF":
		return 0
	case "BHD", "IQD", "JOD", "KWD", "LYD", "OMR", "TND":
		return 3
	default:
		return 2
	}
}

// ParseMoneyCents parses a decimal string ("29.35", "-5.50", "1999") into minor units
// using exact integer arithmetic with zero floating-point rounding drift.
func ParseMoneyCents(s string, currencyCode string) currency.Cents {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = strings.TrimPrefix(s, "-")
	} else if strings.HasPrefix(s, "+") {
		s = strings.TrimPrefix(s, "+")
	}
	dp := CurrencyDecimals(currencyCode)
	wholeStr, fracStr, hasDot := strings.Cut(s, ".")
	if wholeStr == "" {
		wholeStr = "0"
	}
	whole, err := strconv.ParseInt(wholeStr, 10, 64)
	if err != nil {
		return 0
	}
	scale := int64(1)
	for i := 0; i < dp; i++ {
		scale *= 10
	}
	var frac int64
	if hasDot && dp > 0 {
		if len(fracStr) > dp {
			roundDigit := fracStr[dp]
			fracStr = fracStr[:dp]
			frac, _ = strconv.ParseInt(fracStr, 10, 64)
			if roundDigit >= '5' {
				frac++
			}
		} else {
			for len(fracStr) < dp {
				fracStr += "0"
			}
			frac, _ = strconv.ParseInt(fracStr, 10, 64)
		}
	}
	total := whole*scale + frac
	if neg {
		total = -total
	}
	return currency.Cents(total)
}

// FormatMoneyDecimal formats minor units back to a decimal string ("850", "USD" -> "8.50").
func FormatMoneyDecimal(c currency.Cents, currencyCode string) string {
	dp := CurrencyDecimals(currencyCode)
	neg := c < 0
	v := int64(c)
	if neg {
		v = -v
	}
	if dp == 0 {
		if neg {
			return "-" + strconv.FormatInt(v, 10)
		}
		return strconv.FormatInt(v, 10)
	}
	scale := int64(1)
	for i := 0; i < dp; i++ {
		scale *= 10
	}
	whole := v / scale
	frac := v % scale
	out := fmt.Sprintf("%d.%0*d", whole, dp, frac)
	if neg {
		return "-" + out
	}
	return out
}

func stringifyID(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatInt(int64(x), 10)
	case int64:
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	default:
		return ""
	}
}

func stripGID(id string) string {
	if i := strings.LastIndexByte(id, '/'); i >= 0 {
		return id[i+1:]
	}
	return id
}

// NormalizeProduct normalizes a Shopify REST webhook or GraphQL product payload.
func NormalizeProduct(raw []byte, defaultCurrency string) (hub.CanonicalProduct, error) {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var p struct {
		ID          any    `json:"id"`
		AdminGID    string `json:"admin_graphql_api_id"`
		Title       string `json:"title"`
		Handle      string `json:"handle"`
		BodyHTML    string `json:"body_html"`
		Description string `json:"descriptionHtml"`
		Status      string `json:"status"`
		Currency    string `json:"currency"`
		Images      []struct {
			Src string `json:"src"`
			URL string `json:"url"`
		} `json:"images"`
		Variants []struct {
			ID                  any     `json:"id"`
			AdminGID            string  `json:"admin_graphql_api_id"`
			Title               string  `json:"title"`
			SKU                 string  `json:"sku"`
			Barcode             string  `json:"barcode"`
			Price               string  `json:"price"`
			CompareAtPrice      *string `json:"compare_at_price"`
			InventoryQuantity   *int    `json:"inventory_quantity"`
			InventoryManagement *string `json:"inventory_management"`
		} `json:"variants"`
	}
	if err := dec.Decode(&p); err != nil {
		return hub.CanonicalProduct{}, err
	}
	cur := defaultCurrency
	if p.Currency != "" {
		cur = p.Currency
	}
	if cur == "" {
		cur = "USD"
	}
	extID := stripGID(p.AdminGID)
	if extID == "" {
		extID = stripGID(stringifyID(p.ID))
	}
	status := strings.ToLower(strings.TrimSpace(p.Status))
	available := status == "" || status == "active"
	hidden := status == "archived" || status == "draft"

	images := make([]string, 0, len(p.Images))
	for _, img := range p.Images {
		if img.Src != "" {
			images = append(images, img.Src)
		} else if img.URL != "" {
			images = append(images, img.URL)
		}
	}

	vars := make([]hub.CanonicalVariant, 0, len(p.Variants))
	var firstSKU string
	var minPrice currency.Cents
	totalInv := 0
	anyManaged := false
	for i, v := range p.Variants {
		vExt := stripGID(v.AdminGID)
		if vExt == "" {
			vExt = stripGID(stringifyID(v.ID))
		}
		price := ParseMoneyCents(v.Price, cur)
		var msrp currency.Cents
		if v.CompareAtPrice != nil && *v.CompareAtPrice != "" {
			msrp = ParseMoneyCents(*v.CompareAtPrice, cur)
		}
		managed := v.InventoryQuantity != nil
		if v.InventoryManagement != nil && *v.InventoryManagement == "" {
			managed = false
		}
		inv := 0
		if v.InventoryQuantity != nil {
			inv = *v.InventoryQuantity
		}
		if managed {
			anyManaged = true
			totalInv += inv
		}
		if firstSKU == "" && v.SKU != "" {
			firstSKU = v.SKU
		}
		if i == 0 || price < minPrice {
			minPrice = price
		}
		vAvail := available && (!managed || inv > 0)
		vars = append(vars, hub.CanonicalVariant{
			ExternalId:  vExt,
			SKU:         strings.TrimSpace(v.SKU),
			Name:        strings.TrimSpace(v.Title),
			PriceCents:  price,
			MSRPCents:   msrp,
			Available:   vAvail,
			ManageStock: managed,
			Inventory:   inv,
			Barcode:     strings.TrimSpace(v.Barcode),
		})
	}
	desc := p.BodyHTML
	if desc == "" {
		desc = p.Description
	}
	return hub.CanonicalProduct{
		ExternalId:  extID,
		Slug:        strings.TrimSpace(p.Handle),
		SKU:         firstSKU,
		Name:        strings.TrimSpace(p.Title),
		Description: desc,
		Currency:    strings.ToUpper(cur),
		PriceCents:  minPrice,
		Available:   available,
		Hidden:      hidden,
		ManageStock: anyManaged,
		Inventory:   totalInv,
		Images:      images,
		Variants:    vars,
	}, nil
}

// NormalizeOrder normalizes a Shopify order webhook payload into hub.CanonicalOrder.
func NormalizeOrder(raw []byte, defaultCurrency string, forcePaid bool) (hub.CanonicalOrder, error) {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var o struct {
		ID                any    `json:"id"`
		AdminGID          string `json:"admin_graphql_api_id"`
		OrderNumber       any    `json:"order_number"`
		Name              string `json:"name"`
		Email             string `json:"email"`
		ContactEmail      string `json:"contact_email"`
		Currency          string `json:"currency"`
		TotalPrice        string `json:"total_price"`
		TotalDiscounts    string `json:"total_discounts"`
		FinancialStatus   string `json:"financial_status"`
		FulfillmentStatus string `json:"fulfillment_status"`
		CancelledAt       string `json:"cancelled_at"`
		BuyerAcceptsMktg  *bool  `json:"buyer_accepts_marketing"`
		Customer          *struct {
			ID    any    `json:"id"`
			Email string `json:"email"`
		} `json:"customer"`
		DiscountCodes []struct {
			Code string `json:"code"`
		} `json:"discount_codes"`
		LineItems []struct {
			ID        any    `json:"id"`
			ProductID any    `json:"product_id"`
			VariantID any    `json:"variant_id"`
			SKU       string `json:"sku"`
			Title     string `json:"title"`
			Quantity  int    `json:"quantity"`
			Price     string `json:"price"`
		} `json:"line_items"`
		Refunds []struct {
			Transactions []struct {
				Amount string `json:"amount"`
			} `json:"transactions"`
		} `json:"refunds"`
	}
	if err := dec.Decode(&o); err != nil {
		return hub.CanonicalOrder{}, err
	}
	cur := o.Currency
	if cur == "" {
		cur = defaultCurrency
	}
	if cur == "" {
		cur = "USD"
	}
	extID := stripGID(o.AdminGID)
	if extID == "" {
		extID = stripGID(stringifyID(o.ID))
	}
	fin := strings.ToLower(o.FinancialStatus)
	paid := forcePaid || fin == "paid" || fin == "partially_refunded" || fin == "refunded"
	status := "open"
	if o.CancelledAt != "" || fin == "voided" {
		status = "cancelled"
	} else if strings.ToLower(o.FulfillmentStatus) == "fulfilled" {
		status = "completed"
	}
	email := o.Email
	if email == "" {
		email = o.ContactEmail
	}
	custExt := ""
	if o.Customer != nil {
		custExt = stripGID(stringifyID(o.Customer.ID))
		if email == "" {
			email = o.Customer.Email
		}
	}
	codes := make([]string, 0, len(o.DiscountCodes))
	for _, dc := range o.DiscountCodes {
		if dc.Code != "" {
			codes = append(codes, dc.Code)
		}
	}
	lines := make([]hub.CanonicalOrderLine, 0, len(o.LineItems))
	for _, li := range o.LineItems {
		unit := ParseMoneyCents(li.Price, cur)
		lines = append(lines, hub.CanonicalOrderLine{
			ExternalId:        stringifyID(li.ID),
			ExternalProductId: stringifyID(li.ProductID),
			ExternalVariantId: stringifyID(li.VariantID),
			SKU:               strings.TrimSpace(li.SKU),
			Name:              li.Title,
			Quantity:          li.Quantity,
			UnitPriceCents:    unit,
			TotalCents:        unit * currency.Cents(li.Quantity),
		})
	}
	var refunded currency.Cents
	for _, rf := range o.Refunds {
		for _, tx := range rf.Transactions {
			refunded += ParseMoneyCents(tx.Amount, cur)
		}
	}
	consent := ""
	if o.BuyerAcceptsMktg != nil {
		if *o.BuyerAcceptsMktg {
			consent = "subscribed"
		} else {
			consent = "unsubscribed"
		}
	}
	num := stringifyID(o.OrderNumber)
	if num == "" {
		num = strings.TrimPrefix(o.Name, "#")
	}
	return hub.CanonicalOrder{
		ExternalId:       extID,
		Number:           num,
		Status:           status,
		Paid:             paid,
		Currency:         strings.ToUpper(cur),
		TotalCents:       ParseMoneyCents(o.TotalPrice, cur),
		DiscountCents:    ParseMoneyCents(o.TotalDiscounts, cur),
		RefundedCents:    refunded,
		CustomerExternal: custExt,
		Email:            strings.ToLower(strings.TrimSpace(email)),
		CouponCodes:      codes,
		Lines:            lines,
		MarketingConsent: consent,
	}, nil
}

// NormalizeCustomer normalizes a Shopify customer or consent webhook payload.
func NormalizeCustomer(raw []byte) (hub.CanonicalCustomer, error) {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var c struct {
		ID             any    `json:"id"`
		CustomerID     any    `json:"customer_id"`
		AdminGID       string `json:"admin_graphql_api_id"`
		Email          string `json:"email"`
		EmailAddress   string `json:"email_address"`
		FirstName      string `json:"first_name"`
		LastName       string `json:"last_name"`
		EmailMarketing *struct {
			State            string `json:"state"`
			ConsentUpdatedAt string `json:"consent_updated_at"`
		} `json:"email_marketing_consent"`
	}
	if err := dec.Decode(&c); err != nil {
		return hub.CanonicalCustomer{}, err
	}
	extID := stripGID(c.AdminGID)
	if extID == "" {
		extID = stripGID(stringifyID(c.ID))
	}
	if extID == "" {
		extID = stripGID(stringifyID(c.CustomerID))
	}
	email := c.Email
	if email == "" {
		email = c.EmailAddress
	}
	consent := "unknown"
	updatedAt := ""
	if c.EmailMarketing != nil {
		switch strings.ToLower(c.EmailMarketing.State) {
		case "subscribed":
			consent = "subscribed"
		case "unsubscribed":
			consent = "unsubscribed"
		}
		updatedAt = c.EmailMarketing.ConsentUpdatedAt
	}
	return hub.CanonicalCustomer{
		ExternalId:       extID,
		Email:            strings.ToLower(strings.TrimSpace(email)),
		FirstName:        strings.TrimSpace(c.FirstName),
		LastName:         strings.TrimSpace(c.LastName),
		MarketingConsent: consent,
		ConsentUpdatedAt: updatedAt,
	}, nil
}
