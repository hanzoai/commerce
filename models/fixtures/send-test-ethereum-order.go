package fixtures

import (
	"strings"

	"github.com/gin-gonic/gin"

	"appengine/urlfetch"

	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/types/country"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/json"
	"hanzo.io/util/log"
)

var SendTestEthereumOrder = New("send-test-ethererum-order", func(c *gin.Context) {
	org := Organization(c).(*organization.Organization)
	ctx := org.Context()

	db := getNamespaceDb(c)

	u := UserCustomer(c)

	ord := order.New(db)
	ord.UserId = u.Id()
	ord.ShippingAddress.Name = "Jackson Shirts"
	ord.ShippingAddress.Line1 = "1234 Kansas Drive"
	ord.ShippingAddress.City = "Overland Park"

	ctr, _ := country.FindByISO3166_2("US")
	sd, _ := ctr.FindSubDivision("Kansas")

	ord.ShippingAddress.State = sd.Code
	ord.ShippingAddress.Country = ctr.Codes.Alpha2
	ord.ShippingAddress.PostalCode = "66212"

	ord.Currency = currency.ETH
	ord.Subtotal = currency.Cents(100)
	ord.Contribution = true

	log.Info("Sending Test Order", c)
	client := urlfetch.Client(ctx)
	if res, err := client.Post("https://api.hanzo.io/authorize/", "application/json", strings.NewReader(json.Encode(ord))); err != nil {
		panic(err)
	} else {
		log.Info("Geth Node Response: %v", res, c)
	}
})
