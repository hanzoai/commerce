package ads

import (
	"hanzo.io/models/types/currency"
)

// add some functionality to fetch these values from counters
type StatsWeCareAbout struct {
	Clicks      int64          `json:"clicks" datastore:"-"`
	Impressions int64          `json:"impressions" datastore:"-"`
	Conversions int64          `json:"conversions" datastore:"-"`
	TotalSpend  currency.Cents `json:"totalSpend" datastore:"-"`
	Currency    currency.Type  `json:"currency" datastore:"-"`
}
