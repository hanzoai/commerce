package commission

import (
	"hanzo.io/models/types/currency"
)

type Commission struct {
	Minimum currency.Cents `json:"minimum,omitempty"`
	Percent float64        `json:"percent,omitempty"`
	Flat    currency.Cents `json:"flat,omitempty"`
}
