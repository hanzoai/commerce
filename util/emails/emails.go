package emails

import (
	"encoding/gob"
	"strconv"
	"strings"

	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/product"
	"crowdstart.com/models/types/country"
	"crowdstart.com/models/user"
	"crowdstart.com/util/log"

	"appengine"

	mandrill "crowdstart.com/thirdparty/mandrill/tasks"
)

func init() {
	gob.Register([]map[string]interface{}{})
}

func SendOrderConfirmationEmail(ctx appengine.Context, org *organization.Organization, ord *order.Order, usr *user.User) {
	conf := org.Email.OrderConfirmation.Config(org)
	if !conf.Enabled || org.Mandrill.APIKey == "" {
		log.Debug("Skip Email", ctx)
		return
	}

	// From
	fromName := conf.FromName
	fromEmail := conf.FromEmail

	// To
	toEmail := usr.Email
	toName := usr.Name()

	prod := product.New(ord.Db)
	prod.GetById(ord.Items[0].ProductId)

	// Subject, HTML

	// order.number
	// order.items.productName
	// order.items.quantity
	// order.items.displayPrice
	// order.displaySubtotal
	// order.displayDiscount
	// order.displayTax
	// order.displayShipping
	// order.currency
	// order.displayTotal
	// order.shippingAddress.line1
	// order.shippingAddress.line2
	// order.shippingAddress.postalCode
	// order.shippingAddress.state
	// order.shippingAddress.country
	// order.orderDay
	// order.orderMonthName
	// order.orderYear
	subject := conf.Subject

	currencyCode := strings.ToUpper(ord.Currency.Code())
	countryCode := country.ByISOCodeISO3166_2[ord.ShippingAddress.Country].ISO3166OneEnglishShortNameReadingOrder
	items := make([]map[string]interface{}, len(ord.Items))
	vars := map[string]interface{}{
		"order": map[string]interface{}{
			"number":          ord.DisplayId(),
			"displaySubtotal": ord.DisplaySubtotal(),
			"displayDiscount": ord.DisplayDiscount(),
			"displayTax":      ord.DisplayTax(),
			"displayShipping": ord.DisplayShipping(),
			"displayTotal":    ord.DisplayTotal(),
			"currency":        currencyCode,
			"items":           items,
			"shippingAddress": map[string]interface{}{
				"line1":      ord.ShippingAddress.Line1,
				"line2":      ord.ShippingAddress.Line2,
				"postalCode": ord.ShippingAddress.PostalCode,
				"state":      ord.ShippingAddress.State,
				"country":    countryCode,
			},
		},
		"ORDER_NUMBER":                      ord.DisplayId(),
		"ORDER_DISPLAY_SUBTOTAL":            ord.DisplaySubtotal(),
		"ORDER_DISPLAY_DISCOUNT":            ord.DisplayDiscount(),
		"ORDER_DISPLAY_TAX":                 ord.DisplayTax(),
		"ORDER_DISPLAY_SHIPPING":            ord.DisplayShipping(),
		"ORDER_DISPLAY_TOTAL":               ord.DisplayTotal(),
		"ORDER_CURRENCY":                    currencyCode,
		"ORDER_SHIPPING_ADDRESS_LINE1":      ord.ShippingAddress.Line1,
		"ORDER_SHIPPING_ADDRESS_LINE2":      ord.ShippingAddress.Line2,
		"ORDER_SHIPPING_ADDRESS_POSTALCODE": ord.ShippingAddress.PostalCode,
		"ORDER_SHIPPING_ADDRESS_STATE":      ord.ShippingAddress.State,
		"ORDER_SHIPPING_ADDRESS_COUNTRY":    ord.ShippingAddress.Country,

		"user": map[string]interface{}{
			"firstName": usr.FirstName,
			"lastName":  usr.LastName,
		},

		"USER_FIRSTNAME": usr.FirstName,
		"USER_LASTNAME":  usr.LastName,
	}

	for i, item := range ord.Items {
		items[i] = map[string]interface{}{
			"productName":  item.ProductName,
			"quantity":     item.Quantity,
			"displayPrice": item.DisplayPrice(ord.Currency),
		}

		idx := strconv.Itoa(i)
		vars["ORDER_ITEMS_"+idx+"_PRODUCT_NAME"] = item.ProductName
		vars["ORDER_ITEMS_"+idx+"_QUANTITY"] = item.Quantity
		vars["ORDER_ITEMS_"+idx+"_DISPLAY_PRICE"] = item.DisplayTotalPrice(ord.Currency)
	}

	mandrill.SendTemplate(ctx, "order-confirmation", org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, subject, vars)
}
