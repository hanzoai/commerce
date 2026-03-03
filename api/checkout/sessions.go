package checkout

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	square "github.com/square/square-go-sdk/v3"
	sqcheckout "github.com/square/square-go-sdk/v3/checkout"
	sqpaymentlinks "github.com/square/square-go-sdk/v3/checkout/paymentlinks"
	"github.com/square/square-go-sdk/v3/core"
	"github.com/square/square-go-sdk/v3/option"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/models/coupon"
	"github.com/hanzoai/commerce/util/json/http"
)

type checkoutSessionCustomer struct {
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	Address  string `json:"address"`
	City     string `json:"city"`
	Zip      string `json:"zip"`
}

type checkoutSessionHat struct {
	HatColor  string `json:"hatColor"`
	TextColor string `json:"textColor"`
	Text      string `json:"text"`
	BackText  string `json:"backText"`
	Font      string `json:"font"`
	TextStyle string `json:"textStyle"`
	FlagCode  string `json:"flagCode"`
	Size      string `json:"size"`
}

type checkoutSessionItem struct {
	ID        string             `json:"id"`
	Quantity  int                `json:"quantity"`
	UnitPrice float64            `json:"unitPrice"` // ignored; server computes price
	Hat       checkoutSessionHat `json:"hat"`
}

type checkoutSessionRequest struct {
	Company      string                  `json:"company"`
	ProviderHint string                  `json:"providerHint"`
	Currency     string                  `json:"currency"`
	Tenant       string                  `json:"tenant"`
	Org          string                  `json:"org"`
	Project      string                  `json:"project"`
	Customer     checkoutSessionCustomer `json:"customer"`
	Items        []checkoutSessionItem   `json:"items"`
	SuccessURL   string                  `json:"successUrl"`
	CancelURL    string                  `json:"cancelUrl"`
	CouponCode   string                  `json:"couponCode,omitempty"`
	ReferrerId   string                  `json:"referrerId,omitempty"`
	AffiliateId  string                  `json:"affiliateId,omitempty"`
}

// couponDiscount holds the resolved discount for a checkout session.
type couponDiscount struct {
	Code           string  `json:"code"`
	Type           string  `json:"type"`
	Amount         int     `json:"amount"`
	DiscountCents  int64   `json:"discountCents"`
}

type checkoutSessionResponse struct {
	CheckoutURL   string          `json:"checkoutUrl"`
	SessionID     string          `json:"sessionId"`
	Discount      *couponDiscount `json:"discount,omitempty"`
	OriginalTotal int64           `json:"originalTotal,omitempty"`
	FinalTotal    int64           `json:"finalTotal,omitempty"`
}

// resolveCoupon looks up a coupon by code in the datastore and returns it
// if valid. Returns nil (no error) when couponCode is empty.
func resolveCoupon(c *gin.Context, couponCode string) (*coupon.Coupon, error) {
	if couponCode == "" {
		return nil, nil
	}
	db := datastore.New(c)
	cpn := coupon.New(db)
	ok, err := cpn.Query().Filter("Code_=", strings.ToUpper(strings.TrimSpace(couponCode))).Get()
	if err != nil || !ok {
		return nil, errors.New("coupon not found")
	}
	if !cpn.ValidFor(time.Now()) || !cpn.Redeemable() {
		return nil, errors.New("coupon expired or fully redeemed")
	}
	return cpn, nil
}

// applyDiscount computes the discount amount in cents given a subtotal and coupon.
// Returns the discount in cents (always >= 0).
func applyDiscount(subtotalCents int64, cpn *coupon.Coupon) int64 {
	if cpn == nil {
		return 0
	}
	switch cpn.Type {
	case coupon.Percent:
		// Amount field holds the percentage (e.g. 10 = 10%)
		discount := subtotalCents * int64(cpn.Amount) / 100
		if discount > subtotalCents {
			discount = subtotalCents
		}
		return discount
	case coupon.Flat:
		// Amount field holds cents (e.g. 500 = $5.00)
		discount := int64(cpn.Amount)
		if discount > subtotalCents {
			discount = subtotalCents
		}
		return discount
	default:
		return 0
	}
}

func normalizeColor(color string) string {
	return strings.ToLower(strings.TrimSpace(color))
}

func isWhite(color string) bool {
	c := normalizeColor(color)
	return c == "#fff" || c == "#ffffff" || c == "white" || c == "rgb(255,255,255)"
}

func isBlack(color string) bool {
	c := normalizeColor(color)
	return c == "#000" || c == "#000000" || c == "black" || c == "rgb(0,0,0)"
}

func hatPriceCents(h checkoutSessionHat) int64 {
	// Match storefront pricing logic: $80 only for true white-on-white or black-on-black.
	whiteOnWhite := isWhite(h.HatColor) && isWhite(h.TextColor)
	blackOnBlack := isBlack(h.HatColor) && isBlack(h.TextColor)
	if whiteOnWhite || blackOnBlack {
		return 8000
	}
	return 5000
}

func safeName(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return "MEGA Hat"
	}
	// Square line item name max is limited; keep it tight.
	if len(s) > 60 {
		return s[:60]
	}
	return s
}

func isValidRedirect(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return false
	}
	if u.Host == "" {
		return false
	}
	return true
}

func squareCheckoutClient() (*sqpaymentlinks.Client, string, error) {
	squareEnv := strings.ToLower(strings.TrimSpace(os.Getenv("SQUARE_ENVIRONMENT")))
	isSandbox := squareEnv == "sandbox" || squareEnv == "test"

	baseURL := "https://connect.squareup.com"
	if isSandbox {
		baseURL = "https://connect.squareupsandbox.com"
	}

	// Prefer env-specific vars when present; otherwise fall back to the generic names.
	token := strings.TrimSpace(os.Getenv("SQUARE_ACCESS_TOKEN"))
	locationID := strings.TrimSpace(os.Getenv("SQUARE_LOCATION_ID"))
	if isSandbox {
		if t := strings.TrimSpace(os.Getenv("SQUARE_SANDBOX_ACCESS_TOKEN")); t != "" {
			token = t
		}
		if l := strings.TrimSpace(os.Getenv("SQUARE_SANDBOX_LOCATION_ID")); l != "" {
			locationID = l
		}
	}

	if token == "" || locationID == "" {
		return nil, "", errors.New("square is not configured")
	}

	client := sqpaymentlinks.NewClient(core.NewRequestOptions(
		option.WithToken(token),
		option.WithBaseURL(baseURL),
	))
	return client, locationID, nil
}

// Sessions creates a provider-agnostic, Stripe-like hosted checkout session.
//
// Currently implemented using Square Payment Links (hosted checkout URL).
// When providerHint is "wire", returns wire transfer instructions instead.
func Sessions(c *gin.Context) {
	var req checkoutSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		http.Fail(c, 400, "Invalid request", err)
		return
	}

	if len(req.Items) == 0 {
		http.Fail(c, 400, "No items", errors.New("items is required"))
		return
	}

	if !isValidRedirect(req.SuccessURL) {
		http.Fail(c, 400, "Invalid successUrl", errors.New("successUrl is required"))
		return
	}

	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "USD"
	}
	if currency != "USD" {
		http.Fail(c, 400, "Unsupported currency", errors.New("only USD is supported"))
		return
	}

	// Wire transfer: return instructions URL instead of creating a payment link
	if strings.ToLower(strings.TrimSpace(req.ProviderHint)) == "wire" {
		sessionID := uuid.New().String()
		baseURL := strings.TrimSpace(os.Getenv("BASE_URL"))
		if baseURL == "" {
			baseURL = "https://api.hanzo.ai"
		}
		wireURL := baseURL + "/api/v1/checkout/wire/instructions"

		http.Render(c, 200, checkoutSessionResponse{
			CheckoutURL: wireURL,
			SessionID:   sessionID,
		})
		return
	}

	// Resolve coupon before any external calls so we can fail fast.
	cpn, err := resolveCoupon(c, req.CouponCode)
	if err != nil {
		http.Fail(c, 400, "Invalid coupon", err)
		return
	}

	client, locationID, err := squareCheckoutClient()
	if err != nil {
		http.Fail(c, 500, "Payments are not configured", err)
		return
	}

	lineItems := make([]*square.OrderLineItem, 0, len(req.Items))
	var subtotalCents int64
	for _, it := range req.Items {
		if it.Quantity <= 0 {
			http.Fail(c, 400, "Invalid quantity", fmt.Errorf("quantity must be > 0 for item '%s'", it.ID))
			return
		}

		name := safeName(it.Hat.Text)
		qty := fmt.Sprintf("%d", it.Quantity)
		amount := hatPriceCents(it.Hat)
		cur := square.CurrencyUsd

		subtotalCents += amount * int64(it.Quantity)

		lineItems = append(lineItems, &square.OrderLineItem{
			Name:     &name,
			Quantity: qty,
			BasePriceMoney: &square.Money{
				Amount:   &amount,
				Currency: &cur,
			},
		})
	}

	// Apply coupon discount.
	discountCents := applyDiscount(subtotalCents, cpn)
	finalCents := subtotalCents - discountCents

	referenceID := uuid.New().String()
	order := &square.Order{
		LocationID:   locationID,
		LineItems:    lineItems,
		ReferenceID:  &referenceID,
		TicketName:   nil,
		CustomerID:   nil,
		Fulfillments: nil,
	}

	if discountCents > 0 {
		discountUID := "coupon-discount"
		discountName := strings.ToUpper(cpn.Code())
		discountType := square.OrderLineItemDiscountTypeFixedAmount
		discountScope := square.OrderLineItemDiscountScopeOrder
		cur := square.CurrencyUsd
		order.Discounts = []*square.OrderLineItemDiscount{
			{
				UID:   &discountUID,
				Name:  &discountName,
				Type:  &discountType,
				Scope: &discountScope,
				AmountMoney: &square.Money{
					Amount:   &discountCents,
					Currency: &cur,
				},
			},
		}
	}

	redirectURL := req.SuccessURL
	askShipping := false
	checkoutOptions := &square.CheckoutOptions{
		RedirectURL:           &redirectURL,
		AskForShippingAddress: &askShipping,
	}

	buyerEmail := strings.TrimSpace(req.Customer.Email)
	buyerName := strings.TrimSpace(req.Customer.FullName)
	buyerAddressLine1 := strings.TrimSpace(req.Customer.Address)
	buyerCity := strings.TrimSpace(req.Customer.City)
	buyerZip := strings.TrimSpace(req.Customer.Zip)

	var prePop *square.PrePopulatedData
	if buyerEmail != "" || buyerAddressLine1 != "" {
		prePop = &square.PrePopulatedData{
			BuyerEmail: &buyerEmail,
			BuyerAddress: &square.Address{
				FirstName:                    &buyerName,
				AddressLine1:                 &buyerAddressLine1,
				Locality:                     &buyerCity,
				PostalCode:                   &buyerZip,
				Country:                      nil,
				AdministrativeDistrictLevel1: nil,
			},
		}
	}

	desc := strings.TrimSpace(req.Company)
	if desc == "" {
		desc = "Checkout"
	}
	idempotency := uuid.New().String()

	createReq := &sqcheckout.CreatePaymentLinkRequest{
		IdempotencyKey:   &idempotency,
		Description:      &desc,
		Order:            order,
		CheckoutOptions:  checkoutOptions,
		PrePopulatedData: prePop,
	}

	resp, err := client.Create(c.Request.Context(), createReq)
	if err != nil {
		http.Fail(c, 500, "Failed to create checkout session", err)
		return
	}
	if len(resp.Errors) > 0 {
		http.Fail(c, 500, "Failed to create checkout session", errors.New(resp.Errors[0].String()))
		return
	}
	if resp.PaymentLink == nil || resp.PaymentLink.ID == nil || resp.PaymentLink.URL == nil {
		http.Fail(c, 500, "Failed to create checkout session", errors.New("missing payment link"))
		return
	}

	sessionResp := checkoutSessionResponse{
		CheckoutURL: *resp.PaymentLink.URL,
		SessionID:   *resp.PaymentLink.ID,
	}
	if cpn != nil {
		sessionResp.OriginalTotal = subtotalCents
		sessionResp.FinalTotal = finalCents
		sessionResp.Discount = &couponDiscount{
			Code:          cpn.Code(),
			Type:          string(cpn.Type),
			Amount:        cpn.Amount,
			DiscountCents: discountCents,
		}
	}
	// Publish checkout.started to NATS/JetStream (fire and forget)
	if pub, ok := c.Get("publisher"); ok {
		if p, ok := pub.(*events.Publisher); ok {
			orgName := strings.TrimSpace(req.Org)
			if orgName == "" {
				orgName = strings.TrimSpace(req.Tenant)
			}
			go func() {
				bgCtx := context.Background()
				p.PublishCheckoutStarted(bgCtx, sessionResp.SessionID, orgName, finalCents, currency)
			}()
		}
	}

	http.Render(c, 200, sessionResp)
}
