package analyticsidmap

import (
	"hanzo.io/models/mixin"
)

type AnalyticsIdMap struct {
	mixin.Model

	ClientId     string `json:"clientId"`
	UserId       string `json:"userId"`
	SubscriberId string `json:"subscriberId"`
	GAId         string `json:"gaId"`
}
