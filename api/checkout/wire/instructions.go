package wire

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/thirdparty/kms"
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

// Instructions returns wire transfer instructions for the org.
// GET /api/v1/checkout/wire/instructions
// Public endpoint (no auth needed - people need to see where to send money).
func Instructions(c *gin.Context) {
	org := middleware.GetOrganization(c)

	// Hydrate payment credentials from KMS
	if v, ok := c.Get("kms"); ok {
		if kmsClient, ok := v.(*kms.CachedClient); ok {
			if err := kms.Hydrate(kmsClient, org); err != nil {
				log.Error("KMS hydration failed for org %q: %v", org.Name, err, c)
			}
		}
	}

	w := org.Wire
	if w.BankName == "" && w.AccountHolder == "" {
		http.Fail(c, 404, "Wire transfer not configured", nil)
		return
	}

	http.Render(c, 200, wireInstructionsResponse{
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
