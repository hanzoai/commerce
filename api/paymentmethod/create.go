package paymentmethod

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/thirdparty/paymentmethods/plaid"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"

	. "hanzo.io/thirdparty/paymentmethods"
)

func create(c *gin.Context) {
	usr := middleware.GetUser(c)
	usr.Id()

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
		pm = plaid.New(c, "", "", "", plaid.SandboxEnvironment)
	default:
		http.Fail(c, 500, "Invalid payment type: "+t, ErrorInvalidPaymentMethod)
		return
	}

	out, err := pm.GetPayToken(PaymentMethodParams{req.Token})
	if err != nil {
		http.Fail(c, 500, "Error while creating paykey for: "+t, err)
		return
	}

	usr.PaymentMethods = append(usr.PaymentMethods, out)
	if err := usr.Put(); err != nil {
		http.Fail(c, 400, "Failed to update user", err)
	}
}
