package types

import (
	// "hanzo.io/models/ads/ad"
	"hanzo.io/models/ads/adcampaign"
	"hanzo.io/models/ads/adconfig"
	// "hanzo.io/models/ads/adset"
	"hanzo.io/models/copy"
	"hanzo.io/models/media"
)

type AdConfigParams struct {
	adconfig.AdConfig

	Headlines []copy.Copy   `json:"headlines"`
	Copies    []copy.Copy   `json:"copies"`
	Medias    []media.Media `json:"medias"`
}

type CreateInput struct {
	adcampaign.AdCampaign

	AdConfigs []AdConfigParams `json:"adConfigs"`
}

type CreateOutput struct {
	AdCampaign *adcampaign.AdCampaign
	Entities   []interface{}

	// AdCampaign *adcampaign.AdCampaign
	// AdConfigs  []*adconfig.AdConfig
	// AdSets     []*adset.AdSet
	// Ads        []*ad.Ad

	// Headlines []*copy.Copy
	// Copies    []*copy.Copy
	// Medias    []*media.Media
}
