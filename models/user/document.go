package user

import (
	"strings"
	"time"

	"appengine/search"

	"hanzo.io/models/mixin"
	"hanzo.io/models/types/country"
	"hanzo.io/util/searchpartial"
)

type Document struct {
	// Special Kind Facet
	Kind search.Atom `search:",facet"`

	Id_               string
	Email             search.Atom
	EmailPartials     string
	Username          string
	UsernamePartials  string
	FirstName         string
	FirstNamePartials string
	LastName          string
	LastNamePartials  string
	Phone             string

	BillingAddressLine1       string
	BillingAddressLine2       string
	BillingAddressCity        string
	BillingAddressState       string
	BillingAddressCountryCode string
	BillingAddressCountry     string
	BillingAddressPostalCode  string

	ShippingAddressLine1       string
	ShippingAddressLine2       string
	ShippingAddressCity        string
	ShippingAddressState       string
	ShippingAddressCountryCode string
	ShippingAddressCountry     string
	ShippingAddressPostalCode  string

	CreatedAt time.Time
	UpdatedAt time.Time

	StripeBalanceTransactionId string
	StripeCardId               string
	StripeChargeId             string
	StripeCustomerId           string
	StripeLastFour             string
}

func (d Document) Id() string {
	return string(d.Id_)
}

func (u User) Document() mixin.Document {
	emailUser := strings.Split(u.Email, "@")[0]

	return &Document{
		search.Atom(kind),
		u.Id(),
		search.Atom(u.Email),
		searchpartial.Partials(emailUser) + " " + emailUser,
		u.Username,
		searchpartial.Partials(u.Username),
		u.FirstName,
		searchpartial.Partials(u.FirstName),
		u.LastName,
		searchpartial.Partials(u.LastName),
		u.Phone,

		u.BillingAddress.Line1,
		u.BillingAddress.Line2,
		u.BillingAddress.City,
		u.BillingAddress.State,
		u.BillingAddress.Country,
		country.ByISOCodeISO3166_2[u.BillingAddress.Country].ISO3166OneEnglishShortNameReadingOrder,
		u.BillingAddress.PostalCode,

		u.ShippingAddress.Line1,
		u.ShippingAddress.Line2,
		u.ShippingAddress.City,
		u.ShippingAddress.State,
		u.ShippingAddress.Country,
		country.ByISOCodeISO3166_2[u.ShippingAddress.Country].ISO3166OneEnglishShortNameReadingOrder,
		u.ShippingAddress.PostalCode,

		u.CreatedAt,
		u.UpdatedAt,

		u.Accounts.Stripe.BalanceTransactionId,
		u.Accounts.Stripe.CardId,
		u.Accounts.Stripe.ChargeId,
		u.Accounts.Stripe.CustomerId,
		u.Accounts.Stripe.LastFour,
	}
}
