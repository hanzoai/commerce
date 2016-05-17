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
	countryName := country.ByISOCodeISO3166_2[ord.ShippingAddress.Country].ISO3166OneEnglishShortNameReadingOrder
	stateName := ord.ShippingAddress.State
	if len(stateName) <= 2 {
		stateName = strings.ToUpper(stateName)
	}
	items := make([]map[string]interface{}, len(ord.Items))
	vars := map[string]interface{}{
		"order": map[string]interface{}{
			"number":          ord.DisplayId(),
			"displaysubtotal": ord.DisplaySubtotal(),
			"displaydiscount": ord.DisplayDiscount(),
			"displaytax":      ord.DisplayTax(),
			"displayshipping": ord.DisplayShipping(),
			"displaytotal":    ord.DisplayTotal(),
			"currency":        currencyCode,
			"items":           items,
			"shippingaddress": map[string]interface{}{
				"line1":      ord.ShippingAddress.Line1,
				"line2":      ord.ShippingAddress.Line2,
				"postalcode": ord.ShippingAddress.PostalCode,
				"city":       ord.ShippingAddress.City,
				"state":      stateName,
				"country":    countryName,
			},
			"createdday":       ord.CreatedAt.Day(),
			"createdmonthname": ord.CreatedAt.Month().String(),
			"createdyear":      ord.CreatedAt.Year(),
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
		"ORDER_SHIPPING_ADDRESS_CITY":       ord.ShippingAddress.City,
		"ORDER_SHIPPING_ADDRESS_STATE":      stateName,
		"ORDER_SHIPPING_ADDRESS_COUNTRY":    countryName,
		"ORDER_CREATED_DAY":                 ord.CreatedAt.Day(),
		"ORDER_CREATED_MONTH_NAME":          ord.CreatedAt.Month().String(),
		"ORDER_CREATED_YEAR":                ord.CreatedAt.Year(),

		"user": map[string]interface{}{
			"firstname": usr.FirstName,
			"lastname":  usr.LastName,
		},

		"USER_FIRSTNAME": usr.FirstName,
		"USER_LASTNAME":  usr.LastName,
	}

	for i, item := range ord.Items {
		items[i] = map[string]interface{}{
			"productname":  item.ProductName,
			"quantity":     item.Quantity,
			"displayprice": item.DisplayPrice(ord.Currency),
			"currency":     currencyCode,
		}

		idx := strconv.Itoa(i)
		vars["ORDER_ITEMS_"+idx+"_PRODUCT_NAME"] = item.ProductName
		vars["ORDER_ITEMS_"+idx+"_QUANTITY"] = item.Quantity
		vars["ORDER_ITEMS_"+idx+"_DISPLAY_PRICE"] = item.DisplayTotalPrice(ord.Currency)
		vars["ORDER_ITEMS_"+idx+"_CURRENCY"] = currencyCode
	}

	mandrill.SendTemplate(ctx, "order-confirmation", org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, subject, vars)
}
