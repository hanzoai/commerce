package models

import "errors"

var BuyerEmailOrPhoneRequired = errors.New("Buyer's Email or Phone is required.")

type Buyer struct {
	Email     string
	FirstName string
	LastName  string
	Company   string
	Phone     string
	Notes     string
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
