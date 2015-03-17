package models

import "errors"

var BuyerEmailOrPhoneRequired = errors.New("Buyer's Email or Phone is required.")

type Buyer struct {
	Email     string  `json:"email"`
	FirstName string  `json:"firstName"`
	LastName  string  `json:"lastName"`
	Company   string  `json:"company"`
	Phone     string  `json:"phone"`
	Notes     string  `json:"notes"`
	Address   Address `json:"address"`
}

func (b Buyer) Name() string {
	return b.FirstName + " " + b.LastName
}

func (b Buyer) Validate() (bool, []error) {
	if b.Email != "" && b.Phone != "" {
		return false, []error{BuyerEmailOrPhoneRequired}
	}

	return true, nil
}
