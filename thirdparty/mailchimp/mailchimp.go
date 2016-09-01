package mailchimp

import (
	"time"

	"appengine"
	"appengine/urlfetch"

	"github.com/zeekay/gochimp3"

	"crowdstart.com/models/cart"
	"crowdstart.com/models/mailinglist"
	"crowdstart.com/models/order"
	"crowdstart.com/models/product"
	"crowdstart.com/models/store"
	"crowdstart.com/models/subscriber"
	"crowdstart.com/models/types/client"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/user"
	"crowdstart.com/models/variant"
	"crowdstart.com/util/log"

	. "crowdstart.com/models"
)

func centsToFloat(cents currency.Cents, typ currency.Type) float64 {
	amount := float64(cents)
	if !typ.IsZeroDecimal() {
		// Convert cents to dollars
		amount = amount * 0.01
	}
	return amount
}

func idOrEmail(id, email string) string {
	if id == "" {
		return email
	}
	return id
}

type API struct {
	ctx    appengine.Context
	client *gochimp3.API
}

func New(ctx appengine.Context, apiKey string) *API {
	api := new(API)
	api.ctx = ctx
	api.client = gochimp3.New(apiKey)
	api.client.Transport = &urlfetch.Transport{
		Context:  ctx,
		Deadline: time.Duration(60) * time.Second, // Update deadline to 60 seconds
	}
	api.client.Debug = true
	return api
}

func (api API) Subscribe(ml *mailinglist.MailingList, s *subscriber.Subscriber) error {
	list, err := api.client.GetList(ml.Mailchimp.ListId, nil)
	if err != nil {
		log.Error("Failed to subscribe %v: %v", s, err, api.ctx)
		return err
	}

	status := "subscribed"
	if ml.Mailchimp.DoubleOptin {
		status = "pending"
	}

	req := &gochimp3.MemberRequest{
		EmailAddress: s.Email,
		Status:       status,
		MergeFields:  s.MergeFields(),
		Interests:    make(map[string]interface{}),
		Language:     s.Client.Language,
		VIP:          false,
		Location: &gochimp3.MemberLocation{
			Latitude:    0.0,
			Longitude:   0.0,
			GMTOffset:   0,
			DSTOffset:   0,
			CountryCode: s.Client.Country,
			Timezone:    "",
		},
	}

	// Try to update subscriber, create new member if that fails.
	if _, err := list.UpdateMember(s.Md5(), req); err != nil {
		_, err := list.CreateMember(req)
		return err
	}
	return nil
}

func (api API) SubscribeCustomer(listId string, buy Buyer) error {
	ml := new(mailinglist.MailingList)
	ml.Mailchimp.ListId = listId
	s := &subscriber.Subscriber{
		Email:  buy.Email,
		UserId: idOrEmail(buy.UserId, buy.Email),
		Client: client.Client{
			Country: buy.Address.Country,
		},
	}
	return api.Subscribe(ml, s)
}

func (api API) CreateStore(stor *store.Store) error {
	req := &gochimp3.Store{
		// Required
		ID:           stor.Id(),
		ListID:       stor.Mailchimp.ListId, // Immutable after creation
		Name:         stor.Name,
		CurrencyCode: string(stor.Currency),

		// Optional
		Platform:      "Hanzo",
		Domain:        stor.Domain,
		EmailAddress:  stor.Email,
		PrimaryLocale: "en",
		Timezone:      stor.Timezone,
		Phone:         stor.Phone,
		Address: &gochimp3.Address{
			Address1:     stor.Address.Line1,
			Address2:     stor.Address.Line2,
			City:         stor.Address.City,
			ProvinceCode: stor.Address.State,
			PostalCode:   stor.Address.PostalCode,
			CountryCode:  stor.Address.Country,
		},
	}
	_, err := api.client.CreateStore(req)
	return err
}

func (api API) UpdateStore(stor *store.Store) error {
	req := &gochimp3.Store{
		// Required
		ID:           stor.Id(),
		ListID:       stor.Mailchimp.ListId, // Immutable after creation
		Name:         stor.Name,
		CurrencyCode: string(stor.Currency),

		// Optional
		Platform:      "Hanzo",
		Domain:        stor.Domain,
		EmailAddress:  stor.Email,
		PrimaryLocale: "en",
		Timezone:      stor.Timezone,
		Phone:         stor.Phone,
		Address: &gochimp3.Address{
			Address1:     stor.Address.Line1,
			Address2:     stor.Address.Line2,
			City:         stor.Address.City,
			ProvinceCode: stor.Address.State,
			PostalCode:   stor.Address.PostalCode,
			CountryCode:  stor.Address.Country,
		},
	}
	_, err := api.client.UpdateStore(req)
	return err
}

func (api API) DeleteStore(stor *store.Store) error {
	_, err := api.client.DeleteStore(stor.Id())
	return err
}

func (api API) CreateCart(storeId string, car *cart.Cart) error {
	lines := make([]gochimp3.LineItem, 0)
	for _, line := range car.Items {
		lines = append(lines, gochimp3.LineItem{
			ID:               car.Id() + line.Id(),
			ProductID:        line.ProductId,
			ProductVariantID: line.Id(),
			Quantity:         line.Quantity,
			Price:            centsToFloat(line.Price, car.Currency),
		})
	}

	req := &gochimp3.Cart{
		// Required
		CurrencyCode: string(car.Currency),
		OrderTotal:   centsToFloat(car.Total, car.Currency),
		Customer: gochimp3.Customer{
			// Required
			ID: idOrEmail(car.UserId, car.Email),

			// Optional
			EmailAddress: car.Email,
			OptInStatus:  true,
			Company:      car.Company,
			// FirstName:    "",
			// LastName:     "",
			// OrdersCount:  0,
			// TotalSpent:   0,
			// Address:      gochimp3.Address{},
		},

		Lines: lines,

		// Optional
		ID:          car.Id(),
		TaxTotal:    centsToFloat(car.Tax, car.Currency),
		CampaignID:  car.Mailchimp.CampaignId,
		CheckoutURL: car.Mailchimp.CheckoutUrl,
	}

	stor, err := api.client.GetStore(storeId, nil)
	if err != nil {
		log.Warn("Unable to get mailchimp Store '%s': %v", storeId, err, car.Db.Context)
		return err
	}

	_, err = stor.CreateCart(req)
	return err
}

func (api API) UpdateCart(storeId string, car *cart.Cart) error {
	lines := make([]gochimp3.LineItem, 0)
	for _, line := range car.Items {
		lines = append(lines, gochimp3.LineItem{
			ID:               car.Id() + line.Id(),
			ProductID:        line.ProductId,
			ProductVariantID: line.Id(),
			Quantity:         line.Quantity,
			Price:            centsToFloat(line.Price, car.Currency),
		})
	}

	req := &gochimp3.Cart{
		// Required
		CurrencyCode: string(car.Currency),
		OrderTotal:   centsToFloat(car.Total, car.Currency),
		Customer: gochimp3.Customer{
			// Required
			ID: idOrEmail(car.UserId, car.Email),

			// Optional
			EmailAddress: car.Email,
			OptInStatus:  true,
			Company:      car.Company,
			// FirstName:    "",
			// LastName:     "",
			// OrdersCount:  0,
			// TotalSpent:   0,
			// Address:      gochimp3.Address{},
		},
		Lines: lines,

		// Optional
		ID:          car.Id(),
		TaxTotal:    centsToFloat(car.Tax, car.Currency),
		CampaignID:  car.Mailchimp.CampaignId,
		CheckoutURL: car.Mailchimp.CheckoutUrl,
	}

	stor, err := api.client.GetStore(storeId, nil)
	if err != nil {
		log.Warn("Unable to get mailchimp Store '%s': %v", storeId, err, car.Db.Context)
		return err
	}

	_, err = stor.UpdateCart(req)
	return err
}

func (api API) UpdateOrCreateCart(storeId string, car *cart.Cart) error {
	if err := api.UpdateCart(storeId, car); err != nil {
		return api.CreateCart(storeId, car)
	}
	return nil
}

func (api API) DeleteCart(storeId string, car *cart.Cart) error {
	stor, err := api.client.GetStore(storeId, nil)
	if err != nil {
		log.Warn("Unable to get mailchimp Store '%s': %v", storeId, err, car.Db.Context)
		return err
	}

	_, err = stor.DeleteCart(car.Id())
	return err
}

func (api API) CreateOrder(storeId string, ord *order.Order) error {
	// Fetch user
	usr := user.New(ord.Db)
	if err := usr.GetById(ord.UserId); err != nil {
		return err
	}

	// Create line items
	lines := make([]gochimp3.LineItem, 0)
	for _, line := range ord.Items {
		lines = append(lines, gochimp3.LineItem{
			ID:               ord.Id() + line.Id(),
			ProductID:        line.ProductId,
			ProductVariantID: line.Id(),
			Quantity:         line.Quantity,
			Price:            centsToFloat(line.Price, ord.Currency),
		})
	}

	// Create Order
	req := &gochimp3.Order{
		// Required
		ID:           ord.Id(),
		CurrencyCode: string(ord.Currency),
		OrderTotal:   centsToFloat(ord.Total, ord.Currency),
		Customer: gochimp3.Customer{
			// Required
			ID: usr.Id(),

			// Optional
			EmailAddress: usr.Email,
			OptInStatus:  true,
			Company:      ord.Company,
			FirstName:    usr.FirstName,
			LastName:     usr.LastName,
			// OrdersCount:  1,
			// TotalSpent:   centsToFloat(usr.Total, usr.Currency),
			Address: &gochimp3.Address{
				Address1:     ord.ShippingAddress.Line1,
				Address2:     ord.ShippingAddress.Line2,
				City:         ord.ShippingAddress.City,
				ProvinceCode: ord.ShippingAddress.State,
				PostalCode:   ord.ShippingAddress.PostalCode,
				CountryCode:  ord.ShippingAddress.Country,
			},
		},
		Lines: lines,

		// Optional
		TaxTotal:          centsToFloat(ord.Tax, ord.Currency),
		ShippingTotal:     centsToFloat(ord.Shipping, ord.Currency),
		FinancialStatus:   string(ord.PaymentStatus),
		FulfillmentStatus: string(ord.FulfillmentStatus),
		CampaignID:        ord.Mailchimp.CampaignId,
		TrackingCode:      ord.Mailchimp.TrackingCode,
		BillingAddress: &gochimp3.Address{
			Address1:     ord.BillingAddress.Line1,
			Address2:     ord.BillingAddress.Line2,
			City:         ord.BillingAddress.City,
			ProvinceCode: ord.BillingAddress.State,
			PostalCode:   ord.BillingAddress.PostalCode,
			CountryCode:  ord.BillingAddress.Country,
		},
		ShippingAddress: &gochimp3.Address{
			Address1:     ord.ShippingAddress.Line1,
			Address2:     ord.ShippingAddress.Line2,
			City:         ord.ShippingAddress.City,
			ProvinceCode: ord.ShippingAddress.State,
			PostalCode:   ord.ShippingAddress.PostalCode,
			CountryCode:  ord.ShippingAddress.Country,
		},
		ProcessedAtForeign: ord.CreatedAt,
		CancelledAtForeign: ord.CancelledAt,
		UpdatedAtForeign:   ord.UpdatedAt,
	}

	stor, err := api.client.GetStore(storeId, nil)
	if err != nil {
		log.Warn("Unable to get mailchimp Store '%s': %v", storeId, err, ord.Db.Context)
		return err
	}

	_, err = stor.CreateOrder(req)
	return err
}

func (api API) UpdateOrder(storeId string, ord *order.Order) error {
	// Fetch user
	usr := user.New(ord.Db)
	if err := usr.GetById(ord.UserId); err != nil {
		return err
	}

	// Create line items
	lines := make([]gochimp3.LineItem, 0)
	for _, line := range ord.Items {
		lines = append(lines, gochimp3.LineItem{
			ID:               ord.Id() + line.Id(),
			ProductID:        line.ProductId,
			ProductVariantID: line.Id(),
			Quantity:         line.Quantity,
			Price:            centsToFloat(line.Price, ord.Currency),
		})
	}

	// Create order request
	req := &gochimp3.Order{
		// Required
		ID:           ord.Id(),
		CurrencyCode: string(ord.Currency),
		OrderTotal:   centsToFloat(ord.Total, ord.Currency),
		Customer: gochimp3.Customer{
			// Required
			ID: usr.Id(),

			// Optional
			EmailAddress: usr.Email,
			OptInStatus:  true,
			Company:      ord.Company,
			FirstName:    usr.FirstName,
			LastName:     usr.LastName,
			// OrdersCount:  1,
			// TotalSpent:   centsToFloat(usr.Total, usr.Currency),
			Address: &gochimp3.Address{
				Address1:     ord.ShippingAddress.Line1,
				Address2:     ord.ShippingAddress.Line2,
				City:         ord.ShippingAddress.City,
				ProvinceCode: ord.ShippingAddress.State,
				PostalCode:   ord.ShippingAddress.PostalCode,
				CountryCode:  ord.ShippingAddress.Country,
			},
		},
		Lines: lines,

		// Optional
		TaxTotal:          centsToFloat(ord.Tax, ord.Currency),
		ShippingTotal:     centsToFloat(ord.Shipping, ord.Currency),
		FinancialStatus:   string(ord.PaymentStatus),
		FulfillmentStatus: string(ord.FulfillmentStatus),
		CampaignID:        ord.Mailchimp.CampaignId,
		TrackingCode:      ord.Mailchimp.TrackingCode,
		BillingAddress: &gochimp3.Address{
			Address1:     ord.BillingAddress.Line1,
			Address2:     ord.BillingAddress.Line2,
			City:         ord.BillingAddress.City,
			ProvinceCode: ord.BillingAddress.State,
			PostalCode:   ord.BillingAddress.PostalCode,
			CountryCode:  ord.BillingAddress.Country,
		},
		ShippingAddress: &gochimp3.Address{
			Address1:     ord.ShippingAddress.Line1,
			Address2:     ord.ShippingAddress.Line2,
			City:         ord.ShippingAddress.City,
			ProvinceCode: ord.ShippingAddress.State,
			PostalCode:   ord.ShippingAddress.PostalCode,
			CountryCode:  ord.ShippingAddress.Country,
		},
		ProcessedAtForeign: ord.CreatedAt,
		CancelledAtForeign: ord.CancelledAt,
		UpdatedAtForeign:   ord.UpdatedAt,
	}

	stor, err := api.client.GetStore(storeId, nil)
	if err != nil {
		log.Warn("Unable to get mailchimp Store '%s': %v", storeId, err, ord.Db.Context)
		return err
	}

	_, err = stor.UpdateOrder(req)
	return err
}

func (api API) DeleteOrder(storeId string, ord *order.Order) error {
	stor, err := api.client.GetStore(storeId, nil)
	if err != nil {
		log.Warn("Unable to get mailchimp Store '%s': %v", storeId, err, ord.Db.Context)
		return err
	}

	_, err = stor.DeleteOrder(ord.Id())
	return err
}

func (api API) CreateProduct(storeId string, prod *product.Product) error {
	req := &gochimp3.Product{
		ID:          prod.Id(),
		Title:       prod.Name,
		Description: prod.Description,
		// Handle:      "",
		// ImageURL:    "",
		// Type:        "",
		// URL:         "",
		// Vendor:      "",
		Variants: []gochimp3.Variant{
			gochimp3.Variant{
				// Required
				ID:    prod.Id(),
				Title: prod.Name,

				// Optional
				SKU:               prod.Slug,
				Price:             centsToFloat(prod.Price, prod.Currency),
				InventoryQuantity: prod.Inventory,
				Visibility:        "visible",
				// Backorders:        "",
				// ImageUrl:          "",
				// Url:               "",
			},
		},
		PublishedAtForeign: prod.CreatedAt,
	}

	stor, err := api.client.GetStore(storeId, nil)
	if err != nil {
		log.Warn("Unable to get mailchimp Store '%s': %v", storeId, err, prod.Db.Context)
		return err
	}

	_, err = stor.CreateProduct(req)
	return err
}

func (api API) UpdateProduct(storeId string, prod *product.Product) error {
	req := &gochimp3.Product{
		ID:          prod.Id(),
		Title:       prod.Name,
		Description: prod.Description,
		// Handle:      "",
		// ImageURL:    "",
		// Type:        "",
		// URL:         "",
		// Vendor:      "",
		Variants: []gochimp3.Variant{
			gochimp3.Variant{
				// Required
				ID:    prod.Id(),
				Title: prod.Name,

				// Optional
				SKU:               prod.Slug,
				Price:             centsToFloat(prod.Price, prod.Currency),
				InventoryQuantity: prod.Inventory,
				Visibility:        "visible",
				// Backorders:        "",
				// ImageUrl:          "",
				// Url:               "",
			},
		},
		PublishedAtForeign: prod.CreatedAt,
	}

	stor, err := api.client.GetStore(storeId, nil)
	if err != nil {
		log.Warn("Unable to get mailchimp Store '%s': %v", storeId, err, prod.Db.Context)
		return err
	}

	_, err = stor.UpdateProduct(req)
	return err
}

func (api API) DeleteProduct(storeId string, prod *product.Product) error {
	stor, err := api.client.GetStore(storeId, nil)
	if err != nil {
		log.Warn("Unable to get mailchimp Store '%s': %v", storeId, err, prod.Db.Context)
		return err
	}

	_, err = stor.DeleteProduct(prod.Id())
	return err
}

func (api API) CreateVariant(storeId, productId string, vari *variant.Variant) error {
	req := &gochimp3.Variant{
		// Required
		ID:    vari.Id(),
		Title: vari.Name,

		// Optional
		SKU:               vari.SKU,
		Price:             centsToFloat(vari.Price, vari.Currency),
		InventoryQuantity: vari.Inventory,
		Visibility:        "visible",
		// Backorders:        "",
		// ImageUrl:          "",
		// Url:               "",
	}

	stor, err := api.client.GetStore(storeId, nil)
	if err != nil {
		log.Warn("Unable to get mailchimp Store '%s': %v", storeId, err, vari.Db.Context)
		return err
	}

	prod, err := stor.GetProduct(productId, nil)
	if err != nil {
		log.Warn("Unable to get mailchimp product '%s': %v", productId, err, vari.Db.Context)
		return err
	}

	_, err = prod.CreateVariant(req)
	return err
}

func (api API) UpdateVariant(storeId, productId string, vari *variant.Variant) error {
	req := &gochimp3.Variant{
		// Required
		ID:    vari.Id(),
		Title: vari.Name,

		// Optional
		SKU:               vari.SKU,
		Price:             centsToFloat(vari.Price, vari.Currency),
		InventoryQuantity: vari.Inventory,
		Visibility:        "visible",
		// Backorders:        "",
		// ImageUrl:          "",
		// Url:               "",
	}

	stor, err := api.client.GetStore(storeId, nil)
	if err != nil {
		log.Warn("Unable to get mailchimp Store '%s': %v", storeId, err, vari.Db.Context)
		return err
	}

	prod, err := stor.GetProduct(productId, nil)
	if err != nil {
		log.Warn("Unable to get mailchimp product '%s': %v", productId, err, vari.Db.Context)
		return err
	}

	_, err = prod.UpdateVariant(req)
	return err
}

func (api API) DeleteVariant(storeId, productId string, vari *variant.Variant) error {
	stor, err := api.client.GetStore(storeId, nil)
	if err != nil {
		log.Warn("Unable to get mailchimp Store '%s': %v", storeId, err, vari.Db.Context)
		return err
	}

	prod, err := stor.GetProduct(productId, nil)
	if err != nil {
		log.Warn("Unable to get mailchimp product '%s': %v", productId, err, vari.Db.Context)
		return err
	}

	_, err = prod.DeleteVariant(vari.Id())
	return err
}
