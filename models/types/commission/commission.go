package commission

import (
	"crowdstart.com/models/types/currency"
)

type Commission struct {
	Percent float64        `json:"percent,omitempty"`
	Flat    currency.Cents `json:"flat,omitempty"`
}
