package mailchimp

import (
	"time"

	"appengine"
	"appengine/urlfetch"

	"github.com/zeekay/gochimp/chimp_v3"

	"crowdstart.com/models/cart"
	"crowdstart.com/models/mailinglist"
	"crowdstart.com/models/store"
	"crowdstart.com/models/subscriber"
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

func (api API) CreateCart(storeId string, cart *cart.Cart) error {
	lines := make([]gochimp.LineItem, 0)
	for _, line := range cart.Items {
		lines = append(lines, gochimp.LineItem{
			ID:               line.Id(),
			ProductID:        line.ProductId,
			ProductVariantID: line.VariantId,
			Quantity:         line.Quantity,
			Price:            float64(line.Price),
		})
	}

	req := &gochimp.Cart{
		// Required
		CurrencyCode: string(cart.Currency),
		OrderTotal:   float64(cart.Total),

		Customer: gochimp.Customer{
			// Required
			ID: cart.UserId, //string  `json:"id"`

			// Optional
			EmailAddress: cart.UserEmail,
			OptInStatus:  true,
			Company:      cart.Company,
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
		ID: cart.Id(),

		TaxTotal:    float64(cart.Tax),
		CampaignID:  "",
		CheckoutURL: "",
	}
	stor, err := api.client.GetStore(storeId, nil)
	_, err = stor.CreateCart(req)
	return err
}
