package adcampaign

import (
	"hanzo.io/models/mixin"

	. "hanzo.io/models/ads"
)

type FacebookAdCampaign struct {
}

type AdCampaign struct {
	mixin.Model
	FacebookAdCampaign

	Status Status `json:"status"`
}
