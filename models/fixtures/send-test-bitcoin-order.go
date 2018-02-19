package fixtures

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"google.golang.org/appengine/urlfetch"

	"hanzo.io/api/checkout"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/types/country"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/json"
	"hanzo.io/log"
)

var SendTestBitcoinOrder = New("send-test-bitcoin-order", func(c *context.Context) {
	org := Organization(c).(*organization.Organization)
	accessToken := org.MustGetTokenByName("test-published-key")

	ctx := org.Db.Context

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
	ord.Type = payment.Bitcoin

	ord.Currency = currency.BTC
	ord.Subtotal = currency.Cents(1e5)
	ord.Mode = order.ContributionMode

	ch := checkout.Authorization{
		Order: ord,
	}

	j := json.Encode(ch)

	log.Info("Sending To %s", "https://api.hanzo.io/checkout/authorize/", c)
	log.Info("Sending Test Order: %s", j, c)

	client := urlfetch.Client(ctx)
	req, err := http.NewRequest("POST", "https://api.hanzo.io/checkout/authorize/", strings.NewReader(j))
	if err != nil {
		panic(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", accessToken.String())

	if res, err := client.Do(req); err != nil {
		panic(err)
	} else {
		log.Info("Hanzo Test Response: %v", res, c)
	}
})
