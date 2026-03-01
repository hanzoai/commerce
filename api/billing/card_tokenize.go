package billing

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	stripe "github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/token"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/util/json/http"
)

type cardTokenizeRequest struct {
	Number      string `json:"number"`
	ExpiryMonth string `json:"expiry_month"` // "01"-"12"
	ExpiryYear  string `json:"expiry_year"`  // "2026"
	CVC         string `json:"cvc"`
	Name        string `json:"name,omitempty"`
	Zip         string `json:"zip,omitempty"`
}

type cardTokenizeResponse struct {
	Token       string `json:"token"`
	Brand       string `json:"brand"`
	Last4       string `json:"last4"`
	ExpiryMonth string `json:"expiry_month"`
	ExpiryYear  string `json:"expiry_year"`
	Provider    string `json:"provider"`
}

// TokenizeCard accepts raw card data server-side and returns a provider token.
// Raw PAN is never stored; it is forwarded directly to the configured payment
// provider and discarded.
//
// POST /api/v1/billing/card/tokenize
func TokenizeCard(c *gin.Context) {
	var req cardTokenizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		http.Fail(c, 400, "invalid request body", err)
		return
	}

	num := strings.ReplaceAll(req.Number, " ", "")
	if len(num) < 13 || len(num) > 19 {
		http.Fail(c, 400, "invalid card number", nil)
		return
	}
	if req.ExpiryMonth == "" || req.ExpiryYear == "" {
		http.Fail(c, 400, "expiry_month and expiry_year are required", nil)
		return
	}
	if req.CVC == "" {
		http.Fail(c, 400, "cvc is required", nil)
		return
	}

	if key := os.Getenv("STRIPE_SECRET_KEY"); key != "" {
		resp, err := tokenizeWithStripe(c.Request.Context(), key, req, num)
		if err != nil {
			log.Error("stripe tokenization failed: %v", err)
			http.Fail(c, 502, "card tokenization failed", err)
			return
		}
		c.JSON(200, resp)
		return
	}

	http.Fail(c, 503, "no payment provider configured for card tokenization", nil)
}

func tokenizeWithStripe(ctx context.Context, key string, req cardTokenizeRequest, rawNumber string) (*cardTokenizeResponse, error) {
	stripe.Key = key

	params := &stripe.TokenParams{
		Card: &stripe.CardParams{
			Number:   stripe.String(rawNumber),
			ExpMonth: stripe.String(req.ExpiryMonth),
			ExpYear:  stripe.String(req.ExpiryYear),
			CVC:      stripe.String(req.CVC),
		},
	}
	if req.Name != "" {
		params.Card.Name = stripe.String(req.Name)
	}
	if req.Zip != "" {
		params.Card.AddressZip = stripe.String(req.Zip)
	}

	tok, err := token.New(params)
	if err != nil {
		return nil, fmt.Errorf("stripe token creation: %w", err)
	}

	last4, brand, expMonth, expYear := "", "", "", ""
	if card := tok.Card; card != nil {
		last4 = card.Last4
		brand = string(card.Brand)
		expMonth = fmt.Sprintf("%02d", card.ExpMonth)
		expYear = fmt.Sprintf("%d", card.ExpYear)
	}

	return &cardTokenizeResponse{
		Token:       tok.ID,
		Brand:       brand,
		Last4:       last4,
		ExpiryMonth: expMonth,
		ExpiryYear:  expYear,
		Provider:    "stripe",
	}, nil
}
