package review

import "crowdstart.com/models/mixin"

type Review struct {
	mixin.Model

	Id string `json:"id"`

	UserId string `json:"userId"`

	ProductId string `json:"productId"`
	VariantId string `json:"variantId"`

	Name    string `json:"name"`
	Comment string `json:"comment"`
	Rating  int    `json:"rating"`

	Enabled bool `json:"-"`
}
