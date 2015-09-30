package payment

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/paypal"
	"crowdstart.com/util/json/http"
)

type PayKeyResponse struct {
	PayKey string `json:"payKey"`
}

func PayPalPayKey(c *gin.Context) {
	org, ord := getOrganizationAndOrder(c)
	if ord == nil {
		http.Fail(c, 404, "Failed to retrieve order", OrderDoesNotExist)
		return
	}

	ord.Type = "paypal"

	pay, usr, err := authorize(c, org, ord)

	if err != nil {
		http.Fail(c, 500, "Error during authorize", err)
		return
	}

	ctx := org.Db.Context
	client := paypal.New(ctx)

	payKey, err := client.GetPayKey(pay, usr, org)
	if err != nil {
		ord.Status = order.Cancelled
		pay.Status = payment.Cancelled
		ord.MustPut()
		pay.MustPut()

		http.Fail(c, 500, "Paypal Error", err)
		return
	}

	pay.Account.PayPalAccount.PayKey = payKey
	pay.MustPut()

	payKeyResponse := PayKeyResponse{payKey}

	http.Render(c, 200, payKeyResponse)
}

func PayPalConfirm(c *gin.Context) {
}

func PayPalCancel(c *gin.Context) {
}
