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

type Source struct {
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

func (s Source) Card() *stripe.CardParams {
	card := stripe.CardParams{}
	card.Name = s.FirstName + " " + s.LastName
	card.Number = s.Number
	card.Month = s.Month
	card.Year = s.Year
	card.CVC = s.CVC
	card.Address1 = s.Address.Line1
	card.Address2 = s.Address.Line2
	card.City = s.Address.City
	card.State = s.Address.State
	card.Zip = s.Address.PostalCode
	card.Country = s.Address.Country
	return &card
}

func (s Source) Buyer() models.Buyer {
	buyer := models.Buyer{}
	buyer.FirstName = s.FirstName
	buyer.LastName = s.LastName
	buyer.Email = s.Email
	buyer.Phone = s.Phone
	buyer.Company = s.Company
	buyer.Notes = s.Notes
	return buyer
}

type AuthReq struct {
	Source Source       `json:"buyer"`
	Order  *order.Order `json:"order"`
	UserId string       `json:"userId"`
}
