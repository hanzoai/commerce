package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/order"
	"hanzo.io/models/types/country"
	"hanzo.io/models/types/currency"
)

var EthereumOrder = New("send-test-ethererum-order", func(c *gin.Context) *order.Order {
	db := getNamespaceDb(c)

	u := UserCustomer(c)
	Coupon(c)

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

	return ord
})
