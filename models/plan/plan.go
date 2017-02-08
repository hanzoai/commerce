package plan

import "hanzo.io/models/mixin"

type Interval string

const (
	Year  Interval = "year"
	Month          = "month"
)

type Plan struct {
	mixin.Model

	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       int      `json:"price"`
	Interval    Interval `json:"interval"`
}
