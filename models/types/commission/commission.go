package commission

import (
	"crowdstart.com/models/types/currency"
)

type Type string

const (
	Percent Type = "percent"
	Flat         = "flat"
)

type Commission struct {
	Type    Type           `json:"type"`
	Percent float64        `json:"percent,omitempty"`
	Flat    currency.Cents `json:"flat,omitempty"`
}
