package models

type Contribution struct {
	Id            string
	Email         string
	FundingDate   string
	PaymentMethod string
	Perk          Perk
	Status        string
}
