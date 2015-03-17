package payment

import (
	"crowdstart.io/models2"
	"crowdstart.io/models2/order"
	"crowdstart.io/thirdparty/stripe2"
)

type SourceType string

const (
	SourceCard   SourceType = "card"
	SourcePayPal            = "paypal"
	SourceAffirm            = "affirm"
)

type Buyer struct {
	Type SourceType `json:"type"`

	// Buyer
	Email     string         `json:"email"`
	FirstName string         `json:"firstName"`
	LastName  string         `json:"firstName"`
	Company   string         `json:"company"`
	Address   models.Address `json:"address"`
	Notes     string         `json:"notes"`

	// Card
	Number string `json:"number"`
	Month  string `json:"month"`
	CVC    string `json:"cvc"`
	Phone  string `json:"phone"`
	Year   string `json:"year"`
}

func (b Buyer) Card() *stripe.CardParams {
	card := stripe.CardParams{}
	card.Name = b.FirstName + " " + b.LastName
	card.Number = b.Number
	card.Month = b.Month
	card.Year = b.Year
	card.CVC = b.CVC
	card.Address1 = b.Address.Line1
	card.Address2 = b.Address.Line2
	card.City = b.Address.City
	card.State = b.Address.State
	card.Zip = b.Address.PostalCode
	card.Country = b.Address.Country
	return &card
}

func (b Buyer) Buyer() models.Buyer {
	buyer := models.Buyer{}
	buyer.FirstName = b.FirstName
	buyer.LastName = b.LastName
	buyer.Email = b.Email
	buyer.Phone = b.Phone
	buyer.Company = b.Company
	buyer.Notes = b.Notes
	return buyer
}

type AuthReq struct {
	Buyer Buyer        `json:"buyer"`
	Order *order.Order `json:"order"`
}
