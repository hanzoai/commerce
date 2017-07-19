package emails

import (
	"encoding/gob"
	"strconv"
	"strings"
	"time"

	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/subscriber"
	"hanzo.io/models/token"
	"hanzo.io/models/types/country"
	"hanzo.io/models/user"
	"hanzo.io/util/log"

	"appengine"

	mandrill "hanzo.io/thirdparty/mandrill/tasks"
)

func init() {
	gob.Register([]map[string]interface{}{})
}

func MandrillEnabled(ctx appengine.Context, org *organization.Organization, conf organization.Email) bool {
	if !conf.Enabled || org.Mandrill.APIKey == "" {
		if !conf.Enabled {
			log.Debug("Mandrill Disabled", ctx)
		}

		if org.Mandrill.APIKey == "" {
			log.Debug("No Mandrill Key", ctx)
		}

		return false
	}

	return true
}

func SendPasswordResetEmail(ctx appengine.Context, org *organization.Organization, usr *user.User, tok *token.Token) {
	conf := org.Email.User.PasswordReset.Config(org)
	if !MandrillEnabled(ctx, org, conf) {
		return
	}

	// From
	fromName := conf.FromName
	fromEmail := conf.FromEmail

	// To
	toEmail := usr.Email
	toName := usr.Name()

	// Subject
	subject := conf.Subject

	// Create Merge Vars
	vars := map[string]interface{}{
		"user": map[string]interface{}{
			"firstname": usr.FirstName,
			"lastname":  usr.LastName,
		},
		"token": map[string]interface{}{
			"id": tok.Id(),
		},

		"USER_FIRSTNAME": usr.FirstName,
		"USER_LASTNAME":  usr.LastName,
		"TOKEN_ID":       tok.Id(),
	}

	// Send Email
	mandrill.SendTemplate(ctx, "password-reset", org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, subject, vars)
}

func SendEmailConfirmedEmail(ctx appengine.Context, org *organization.Organization, usr *user.User) {
	conf := org.Email.User.EmailConfirmation.Config(org)
	if !MandrillEnabled(ctx, org, conf) {
		return
	}

	// From
	fromName := conf.FromName
	fromEmail := conf.FromEmail

	// To
	toEmail := usr.Email
	toName := usr.Name()

	// Subject
	subject := conf.Subject

	// Create Merge Vars
	vars := map[string]interface{}{
		"user": map[string]interface{}{
			"firstname": usr.FirstName,
			"lastname":  usr.LastName,
		},
		"USER_FIRSTNAME": usr.FirstName,
		"USER_LASTNAME":  usr.LastName,
	}

	// Send Email
	mandrill.SendTemplate(ctx, "user-email-confirmed", org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, subject, vars)
}

func SendSubscriberWelcome(ctx appengine.Context, org *organization.Organization, s *subscriber.Subscriber) {
	conf := org.Email.Subscriber.Welcome.Config(org)
	if !MandrillEnabled(ctx, org, conf) {
		return
	}

	// From
	fromName := conf.FromName
	fromEmail := conf.FromEmail

	// To
	toEmail := s.Email
	toName := s.Name()

	// Subject
	subject := conf.Subject

	// Send Email
	mandrill.SendTemplate(ctx, "subscriber-welcome-email", org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, subject, s.Metadata)
}

func SendUserWelcome(ctx appengine.Context, org *organization.Organization, usr *user.User) {
	conf := org.Email.User.Welcome.Config(org)
	if !MandrillEnabled(ctx, org, conf) {
		return
	}

	// From
	fromName := conf.FromName
	fromEmail := conf.FromEmail

	// To
	toEmail := usr.Email
	toName := usr.Name()

	// Subject
	subject := conf.Subject

	// Create Merge Vars
	vars := map[string]interface{}{
		"user": map[string]interface{}{
			"firstname": usr.FirstName,
			"lastname":  usr.LastName,
		},
		"USER_FIRSTNAME": usr.FirstName,
		"USER_LASTNAME":  usr.LastName,
	}

	// Send Email
	mandrill.SendTemplate(ctx, "user-welcome-email", org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, subject, vars)
}

func SendAccountCreationConfirmationEmail(ctx appengine.Context, org *organization.Organization, usr *user.User) {
	conf := org.Email.User.EmailConfirmed.Config(org)
	if !MandrillEnabled(ctx, org, conf) {
		return
	}

	// Create token
	tok := token.New(usr.Db)
	tok.Email = usr.Email
	tok.UserId = usr.Id()
	tok.Expires = time.Now().Add(time.Hour * 24 * 7)

	err := tok.Put()
	if err != nil {
		panic(err)
	}

	// From
	fromName := conf.FromName
	fromEmail := conf.FromEmail

	// To
	toEmail := usr.Email
	toName := usr.Name()

	// Subject
	subject := conf.Subject

	// Create Merge Vars
	vars := map[string]interface{}{
		"user": map[string]interface{}{
			"firstname": usr.FirstName,
			"lastname":  usr.LastName,
		},
		"token": map[string]interface{}{
			"id": tok.Id(),
		},

		"USER_FIRSTNAME": usr.FirstName,
		"USER_LASTNAME":  usr.LastName,
		"TOKEN_ID":       tok.Id(),
	}

	// Send Email
	mandrill.SendTemplate(ctx, "user-email-confirmation", org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, subject, vars)
}

func SendOrderConfirmationEmail(ctx appengine.Context, org *organization.Organization, ord *order.Order, usr *user.User) {
	conf := org.Email.OrderConfirmation.Config(org)
	if !MandrillEnabled(ctx, org, conf) {
		return
	}

	// From
	fromName := conf.FromName
	fromEmail := conf.FromEmail

	// To
	toEmail := usr.Email
	toName := usr.Name()

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
			"displaytax":      ord.DisplayTax(),
			"displayshipping": ord.DisplayShipping(),
			"displaytotal":    ord.DisplayTotal(),
			"currency":        currencyCode,
			"items":           items,
			"shippingaddress": map[string]interface{}{
				"name":       ord.ShippingAddress.Name,
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
		"ORDER_DISPLAY_TAX":                 ord.DisplayTax(),
		"ORDER_DISPLAY_SHIPPING":            ord.DisplayShipping(),
		"ORDER_DISPLAY_TOTAL":               ord.DisplayTotal(),
		"ORDER_CURRENCY":                    currencyCode,
		"ORDER_SHIPPING_ADDRESS_NAME":       ord.ShippingAddress.Name,
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

	if ord.Discount != 0 {
		ordVars := vars["order"].(map[string]interface{})
		ordVars["displaydiscount"] = ord.DisplayDiscount()
		vars["ORDER_DISPLAY_DISCOUNT"] = ord.DisplayDiscount()
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

func SendPartialRefundEmail(ctx appengine.Context, org *organization.Organization, ord *order.Order, usr *user.User, pay *payment.Payment) {
	conf := org.Email.OrderConfirmation.Config(org)
	if !MandrillEnabled(ctx, org, conf) {
		return
	}

	// From
	fromName := conf.FromName
	fromEmail := conf.FromEmail

	// To
	toEmail := usr.Email
	toName := usr.Name()

	// Subject, HTML
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
			"number":           ord.DisplayId(),
			"displaysubtotal":  ord.DisplaySubtotal(),
			"displaytax":       ord.DisplayTax(),
			"displayshipping":  ord.DisplayShipping(),
			"displaytotal":     ord.DisplayTotal(),
			"displayrefunded":  ord.DisplayRefunded(),
			"displayremaining": ord.DisplayRemaining(),
			"currency":         currencyCode,
			"items":            items,
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
		"ORDER_DISPLAY_TAX":                 ord.DisplayTax(),
		"ORDER_DISPLAY_SHIPPING":            ord.DisplayShipping(),
		"ORDER_DISPLAY_TOTAL":               ord.DisplayTotal(),
		"ORDER_DISPLAY_REFUNDED":            ord.DisplayRefunded(),
		"ORDER_DISPLAY_REMAINING":           ord.DisplayRemaining(),
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

		"payment": map[string]interface{}{
			"lastfour": pay.Account.LastFour,
		},

		"PAYMENT_LASTFOUR": pay.Account.LastFour,
	}

	if ord.Discount != 0 {
		ordVars := vars["order"].(map[string]interface{})
		ordVars["displaydiscount"] = ord.DisplayDiscount()
		vars["ORDER_DISPLAY_DISCOUNT"] = ord.DisplayDiscount()
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

	mandrill.SendTemplate(ctx, "order-partially-refunded", org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, subject, vars)
}

func SendFullRefundEmail(ctx appengine.Context, org *organization.Organization, ord *order.Order, usr *user.User, pay *payment.Payment) {
	conf := org.Email.OrderConfirmation.Config(org)
	if !MandrillEnabled(ctx, org, conf) {
		return
	}

	// From
	fromName := conf.FromName
	fromEmail := conf.FromEmail

	// To
	toEmail := usr.Email
	toName := usr.Name()

	// Subject, HTML
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
			"number":           ord.DisplayId(),
			"displaysubtotal":  ord.DisplaySubtotal(),
			"displaytax":       ord.DisplayTax(),
			"displayshipping":  ord.DisplayShipping(),
			"displaytotal":     ord.DisplayTotal(),
			"displayrefunded":  ord.DisplayRefunded(),
			"displayremaining": ord.DisplayRemaining(),
			"currency":         currencyCode,
			"items":            items,
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
		"ORDER_DISPLAY_TAX":                 ord.DisplayTax(),
		"ORDER_DISPLAY_SHIPPING":            ord.DisplayShipping(),
		"ORDER_DISPLAY_TOTAL":               ord.DisplayTotal(),
		"ORDER_DISPLAY_REFUNDED":            ord.DisplayRefunded(),
		"ORDER_DISPLAY_REMAINING":           ord.DisplayRemaining(),
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

		"payment": map[string]interface{}{
			"lastfour": pay.Account.LastFour,
		},

		"PAYMENT_LASTFOUR": pay.Account.LastFour,
	}

	if ord.Discount != 0 {
		ordVars := vars["order"].(map[string]interface{})
		ordVars["displaydiscount"] = ord.DisplayDiscount()
		vars["ORDER_DISPLAY_DISCOUNT"] = ord.DisplayDiscount()
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

	mandrill.SendTemplate(ctx, "order-refunded", org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, subject, vars)
}

func SendFulfillmentEmail(ctx appengine.Context, org *organization.Organization, ord *order.Order, usr *user.User, pay *payment.Payment) {
	conf := org.Email.OrderConfirmation.Config(org)
	if !MandrillEnabled(ctx, org, conf) {
		return
	}

	// From
	fromName := conf.FromName
	fromEmail := conf.FromEmail

	// To
	toEmail := usr.Email
	toName := usr.Name()

	// Subject, HTML
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
			"number":           ord.DisplayId(),
			"displaysubtotal":  ord.DisplaySubtotal(),
			"displaytax":       ord.DisplayTax(),
			"displayshipping":  ord.DisplayShipping(),
			"displaytotal":     ord.DisplayTotal(),
			"displayrefunded":  ord.DisplayRefunded(),
			"displayremaining": ord.DisplayRemaining(),
			"currency":         currencyCode,
			"items":            items,
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
			"fulfillment": map[string]interface{}{
				"trackingnumber": "",
				"service":        ord.Fulfillment.Service,
				"carrier":        ord.Fulfillment.Carrier,
			},
		},
		"ORDER_NUMBER":                      ord.DisplayId(),
		"ORDER_DISPLAY_SUBTOTAL":            ord.DisplaySubtotal(),
		"ORDER_DISPLAY_TAX":                 ord.DisplayTax(),
		"ORDER_DISPLAY_SHIPPING":            ord.DisplayShipping(),
		"ORDER_DISPLAY_TOTAL":               ord.DisplayTotal(),
		"ORDER_DISPLAY_REFUNDED":            ord.DisplayRefunded(),
		"ORDER_DISPLAY_REMAINING":           ord.DisplayRemaining(),
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
		"ORDER_FULFILLMENT_TRACKING_NUMBER": "",
		"ORDER_FULFILLMENT_SERVICE":         ord.Fulfillment.Service,
		"ORDER_FULFILLMENT_CARRIER":         ord.Fulfillment.Carrier,

		"user": map[string]interface{}{
			"firstname": usr.FirstName,
			"lastname":  usr.LastName,
		},

		"USER_FIRSTNAME": usr.FirstName,
		"USER_LASTNAME":  usr.LastName,

		"payment": map[string]interface{}{
			"lastfour": pay.Account.LastFour,
		},

		"PAYMENT_LASTFOUR": pay.Account.LastFour,
	}

	if ord.Discount != 0 {
		ordVars := vars["order"].(map[string]interface{})
		ordVars["displaydiscount"] = ord.DisplayDiscount()
		vars["ORDER_DISPLAY_DISCOUNT"] = ord.DisplayDiscount()
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

	mandrill.SendTemplate(ctx, "order-shipped", org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, subject, vars)
}
