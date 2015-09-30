package payment

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/thirdparty/paypal"
	"crowdstart.com/util/json/http"
)

type PayKeyResponse struct {
	PayKey string `json:"payKey"`
}

func PayKey(c *gin.Context) {
	org, ord := getOrganizationAndOrder(c)
	if ord == nil {
		http.Fail(c, 404, "Failed to retrieve order", OrderDoesNotExist)
		return
	}

	ord.Type = "paypal"
	ctx := org.Db.Context

	pay, usr, err := authorize(c, org, ord)

	if err != nil {
		http.Fail(c, 500, "Error during authorize", err)
		return
	}

	client := paypal.New(ctx)
	payKey, err := client.GetPayKey(pay, usr, org)

	payKeyResponse := PayKeyResponse{payKey}

	http.Render(c, 200, payKeyResponse)
}
