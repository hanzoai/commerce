package wire

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/payment"
	"github.com/hanzoai/commerce/util/json/http"
)

type wireInstructionsResponse struct {
	BankName      string `json:"bankName,omitempty"`
	AccountHolder string `json:"accountHolder,omitempty"`
	RoutingNumber string `json:"routingNumber,omitempty"`
	AccountNumber string `json:"accountNumber,omitempty"`
	SWIFT         string `json:"swift,omitempty"`
	IBAN          string `json:"iban,omitempty"`
	BankAddress   string `json:"bankAddress,omitempty"`
	Reference     string `json:"reference,omitempty"`
	Instructions  string `json:"instructions,omitempty"`
}

// Instructions returns the caller org's wire-transfer instructions.
// GET /v1/checkout/wire/instructions
//
// Fails CLOSED: the org comes ONLY from the authenticated principal (set by the
// identity/token middleware). A request with no resolved org gets a clean 401 —
// never a panic (the previous GetOrganization MustGet 500'd on any org-less
// call after EdgeAuth strips a spoofed X-Org-Id) and never another org's bank
// details. Account/routing/SWIFT are org-specific sensitive data, so there is
// no anonymous read.
func Instructions(c *zip.Ctx) error {
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return http.Fail(c, 401, "Authentication required",
			errors.New("no authenticated organization for wire instructions"))
	}

	// The org is the merchant here, so its own wire details are read.
	payment.Hydrate(c.Forward(), org, "wire") // Forward carries the caller the host's KMS admits

	w := org.Wire
	if w.BankName == "" && w.AccountHolder == "" {
		return http.Fail(c, 404, "Wire transfer not configured", nil)
	}

	return http.Render(c, 200, wireInstructionsResponse{
		BankName:      w.BankName,
		AccountHolder: w.AccountHolder,
		RoutingNumber: w.RoutingNumber,
		AccountNumber: w.AccountNumber,
		SWIFT:         w.SWIFT,
		IBAN:          w.IBAN,
		BankAddress:   w.BankAddress,
		Reference:     w.Reference,
		Instructions:  w.Instructions,
	})
}
