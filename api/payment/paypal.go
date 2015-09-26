package payment

import (
	"crowdstart.com/thirdparty/paypal"
	"github.com/gg2/gin"
)

func PaypalRedirect(c *gin.Context) {
	org, org := getOrganizationAndOrder(c)

	if ord == nil {
		http.Fail(c, 404, "Failed to retrieve order", OrderDoesNotExist)
		return
	}

	// Do capture using order from authorization
	ord, err = paypal.GetPayKey(c, org, ord)
	if err != nil {
		http.Fail(c, 500, "Error during capture", err)
		return
	}
}

func PaypalFail(c *gin.Context) {

}

func PaypalSuccess(c *gin.Context) {

}
