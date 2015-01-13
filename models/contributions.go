package models

type Contribution struct {
	Id            string
	UserId        string
	FundingDate   string
	PaymentMethod string
	Perk          Perk
	Status        string
}
