package mailchimp

import (
	"time"

	"appengine"
	"appengine/urlfetch"

	"github.com/zeekay/gochimp/chimp_v3"

	"crowdstart.com/models/cart"
	"crowdstart.com/models/mailinglist"
	"crowdstart.com/models/order"
	"crowdstart.com/models/product"
	"crowdstart.com/models/store"
	"crowdstart.com/models/subscriber"
	"crowdstart.com/models/variant"
	"crowdstart.com/util/log"
)

type API struct {
	ctx    appengine.Context
	client *gochimp.ChimpAPI
}

func New(ctx appengine.Context, apiKey string) *API {
	api := new(API)
	api.ctx = ctx
	api.client = gochimp.NewChimp(apiKey, true)
	api.client.Transport = &urlfetch.Transport{
		Context:  ctx,
		Deadline: time.Duration(60) * time.Second, // Update deadline to 60 seconds
	}
	return api
}

func (api API) Subscribe(ml *mailinglist.MailingList, s *subscriber.Subscriber) error {
	list, err := api.client.GetList(ml.Mailchimp.Id, nil)
	if err != nil {
		log.Error("Failed to subscribe %v: %v", s, err, api.ctx)
		return err
	}

	status := "subscribed"
	if ml.Mailchimp.DoubleOptin {
		status = "pending"
	}

	req := &gochimp.MemberRequest{
		EmailAddress: s.Email,
		Status:       status,
		MergeFields:  s.MergeFields(),
		Interests:    make(map[string]interface{}),
		Language:     s.Client.Language,
		VIP:          false,
		Location: gochimp.MemberLocation{
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

func (api API) CreateStore(stor *store.Store) error {
	req := &gochimp.Store{
		// Required
		ID:           stor.Id(),
		ListID:       stor.Mailchimp.ListId, // Immutable after creation
		Name:         stor.Name,
		CurrencyCode: "USD",

		// Optional
		Platform:      "Hanzo",
		Domain:        stor.Domain,
		EmailAddress:  stor.Email,
		PrimaryLocale: "en",
		Timezone:      stor.Timezone,
		Phone:         stor.Phone,
		Address: gochimp.Address{
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
	req := &gochimp.Store{
		// Required
		ID:           stor.Id(),
		ListID:       stor.Mailchimp.ListId, // Immutable after creation
		Name:         stor.Name,
		CurrencyCode: "USD",

		// Optional
		Platform:      "Hanzo",
		Domain:        stor.Domain,
		EmailAddress:  stor.Email,
		PrimaryLocale: "en",
		Timezone:      stor.Timezone,
		Phone:         stor.Phone,
		Address: gochimp.Address{
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
	lines := make([]gochimp.LineItem, 0)
	for _, line := range car.Items {
		lines = append(lines, gochimp.LineItem{
			ID:               car.Id() + line.VariantId,
			ProductID:        line.ProductId,
			ProductVariantID: line.VariantId,
			Quantity:         line.Quantity,
			Price:            float64(line.Price),
		})
	}

	req := &gochimp.Cart{
		// Required
		CurrencyCode: string(car.Currency),
		OrderTotal:   float64(car.Total),

		Customer: gochimp.Customer{
			// Required
			ID: car.UserId, //string  `json:"id"`

			// Optional
			EmailAddress: car.UserEmail,
			OptInStatus:  true,
			Company:      car.Company,
			FirstName:    "",
			LastName:     "",
			OrdersCount:  0,
			TotalSpent:   0,
			Address:      gochimp.Address{},
			CreatedAt:    "",
			UpdatedAt:    "",
		},

		Lines: lines,

		// Optional
		ID: car.Id(),

		TaxTotal:    float64(car.Tax),
		CampaignID:  "",
		CheckoutURL: "",
	}
	stor, err := api.client.GetStore(storeId, nil)
	_, err = stor.CreateCart(req)
	return err
}

func (api API) UpdateCart(storeId string, car *cart.Cart) error {
	lines := make([]gochimp.LineItem, 0)
	for _, line := range car.Items {
		lines = append(lines, gochimp.LineItem{
			ID:               car.Id() + line.VariantId,
			ProductID:        line.ProductId,
			ProductVariantID: line.VariantId,
			Quantity:         line.Quantity,
			Price:            float64(line.Price),
		})
	}

	req := &gochimp.Cart{
		// Required
		CurrencyCode: string(car.Currency),
		OrderTotal:   float64(car.Total),

		Customer: gochimp.Customer{
			// Required
			ID: car.UserId, //string  `json:"id"`

			// Optional
			EmailAddress: car.UserEmail,
			OptInStatus:  true,
			Company:      car.Company,
			FirstName:    "",
			LastName:     "",
			OrdersCount:  0,
			TotalSpent:   0,
			Address:      gochimp.Address{},
			CreatedAt:    "",
			UpdatedAt:    "",
		},

		Lines: lines,

		// Optional
		ID: car.Id(),

		TaxTotal:    float64(car.Tax),
		CampaignID:  "",
		CheckoutURL: "",
	}
	stor, err := api.client.GetStore(storeId, nil)
	_, err = stor.UpdateCart(req)
	return err
}

func (api API) DeleteCart(storeId string, car *cart.Cart) error {
	stor, err := api.client.GetStore(storeId, nil)
	_, err = stor.DeleteCart(car.Id())
	return err
}

func (api API) CreateOrder(storeId string, ord *order.Order) error {
	lines := make([]gochimp.LineItem, 0)
	for _, line := range ord.Items {
		lines = append(lines, gochimp.LineItem{
			ID:               ord.Id() + line.VariantId,
			ProductID:        line.ProductId,
			ProductVariantID: line.VariantId,
			Quantity:         line.Quantity,
			Price:            float64(line.Price),
		})
	}

	req := &gochimp.Order{
		// Required
		CurrencyCode: string(ord.Currency),
		OrderTotal:   float64(ord.Total),

		Customer: gochimp.Customer{
			// Required
			ID: ord.UserId, //string  `json:"id"`

			// Optional
			EmailAddress: ord.UserId, // FIXME
			OptInStatus:  true,
			Company:      ord.Company,
			FirstName:    "",
			LastName:     "",
			OrdersCount:  0,
			TotalSpent:   0,
			Address:      gochimp.Address{},
			CreatedAt:    "",
			UpdatedAt:    "",
		},

		Lines: lines,

		// Optional
		ID: ord.Id(),

		TaxTotal:    float64(ord.Tax),
		CampaignID:  "",
		CheckoutURL: "",
	}
	stor, err := api.client.GetStore(storeId, nil)
	_, err = stor.CreateOrder(req)
	return err
}

func (api API) UpdateOrder(storeId string, ord *order.Order) error {
	lines := make([]gochimp.LineItem, 0)
	for _, line := range ord.Items {
		lines = append(lines, gochimp.LineItem{
			ID:               ord.Id() + line.VariantId,
			ProductID:        line.ProductId,
			ProductVariantID: line.VariantId,
			Quantity:         line.Quantity,
			Price:            float64(line.Price),
		})
	}

	req := &gochimp.Order{
		// Required
		CurrencyCode: string(ord.Currency),
		OrderTotal:   float64(ord.Total),

		Customer: gochimp.Customer{
			// Required
			ID: ord.UserId, //string  `json:"id"`

			// Optional
			EmailAddress: ord.UserId, // FIXME
			OptInStatus:  true,
			Company:      ord.Company,
			FirstName:    "",
			LastName:     "",
			OrdersCount:  0,
			TotalSpent:   0,
			Address:      gochimp.Address{},
			CreatedAt:    "",
			UpdatedAt:    "",
		},

		Lines: lines,

		// Optional
		ID: ord.Id(),

		TaxTotal:    float64(ord.Tax),
		CampaignID:  "",
		CheckoutURL: "",
	}
	stor, err := api.client.GetStore(storeId, nil)
	_, err = stor.UpdateOrder(req)
	return err
}

func (api API) DeleteOrder(storeId string, ord *order.Order) error {
	stor, err := api.client.GetStore(storeId, nil)
	_, err = stor.DeleteOrder(ord.Id())
	return err
}

func (api API) CreateProduct(storeId string, prod *product.Product) error {
	req := &gochimp.Product{}
	stor, err := api.client.GetStore(storeId, nil)
	_, err = stor.CreateProduct(req)
	return err
}

func (api API) UpdateProduct(storeId string, prod *product.Product) error {
	req := &gochimp.Product{}
	stor, err := api.client.GetStore(storeId, nil)
	_, err = stor.UpdateProduct(req)
	return err
}

func (api API) DeleteProduct(storeId string, prod *product.Product) error {
	stor, err := api.client.GetStore(storeId, nil)
	_, err = stor.DeleteProduct(prod.Id())
	return err
}

func (api API) CreateVariant(storeId, productId string, vari *variant.Variant) error {
	req := &gochimp.Variant{}
	stor, err := api.client.GetStore(storeId, nil)
	prod, err := stor.GetProduct(productId, nil)
	_, err = prod.CreateVariant(req)
	return err
}

func (api API) UpdateVariant(storeId, productId string, vari *variant.Variant) error {
	req := &gochimp.Variant{}
	stor, err := api.client.GetStore(storeId, nil)
	prod, err := stor.GetProduct(productId, nil)
	_, err = prod.UpdateVariant(req)
	return err
}

func (api API) DeleteVariant(storeId, productId string, vari *variant.Variant) error {
	stor, err := api.client.GetStore(storeId, nil)
	prod, err := stor.GetProduct(productId, nil)
	_, err = prod.DeleteVariant(vari.Id())
	return err
}
