package types

import (
	// "github.com/hanzoai/commerce/models/ads/ad"
	"github.com/hanzoai/commerce/models/ads/adcampaign"
	"github.com/hanzoai/commerce/models/ads/adconfig"
	// "github.com/hanzoai/commerce/models/ads/adset"
	"github.com/hanzoai/commerce/models/copy"
	"github.com/hanzoai/commerce/models/media"
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
