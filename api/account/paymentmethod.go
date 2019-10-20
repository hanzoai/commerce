package account

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/models/paymentmethod"
	"hanzo.io/thirdparty/paymentmethods/plaid"
	"hanzo.io/types/integration"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"

	. "encoding/json"
	. "hanzo.io/thirdparty/paymentmethods"
)

type CreateReq struct {
	PublicToken string     `json:"publicToken"`
	AccountId   string     `json:"accountId"`
	Name        string     `json:"name"`
	Metadata    RawMessage `json:"metadata"`
}

func createPaymentMethod(c *gin.Context) {
	usr := middleware.GetUser(c)
	org := middleware.GetOrganization(c)

	req := &CreateReq{}

	// Decode response body to create new user
	if err := json.Decode(c.Request.Body, req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	var pm PaymentMethod

	t := c.Params.ByName("paymentmethodtype")
	switch t {
	case "plaid":
		in := org.Integrations.FindByType(integration.PlaidType)
		if in == nil {
			http.Fail(c, 500, "Missing plaid credentials: "+t, ErrorMissingCredentials)
			return
		}
		pm = plaid.New(org.Context(), in.Plaid.ClientId, in.Plaid.Secret, in.Plaid.PublicKey, plaid.SandboxEnvironment)
	default:
		http.Fail(c, 500, "Invalid payment type: "+t, ErrorInvalidPaymentMethod)
		return
	}

	out, err := pm.GetPayToken(PaymentMethodParams{req.PublicToken, req.AccountId, req.Metadata})
	if err != nil {
		http.Fail(c, 500, "Error while creating paykey for: "+t, err)
		return
	}

	p := paymentmethod.New(usr.Db)
	p.UserId = usr.Id()
	p.Name = req.Name
	p.PaymentMethodOutput = *out

	if err := p.Create(); err != nil {
		http.Fail(c, 400, "Failed to add payment method", err)
		return
	}

	http.Render(c, 201, p)
}
