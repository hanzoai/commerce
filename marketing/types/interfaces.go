package types

import (
	"hanzo.io/datastore"
	"hanzo.io/models/ads/ad"
	"hanzo.io/models/ads/adcampaign"
	"hanzo.io/models/ads/adconfig"
	"hanzo.io/models/ads/adset"
)

type Runnable interface {
	Create(*datastore.Datastore, CreateInput) (CreateOutput, error)

	StartAdCampaign(*adcampaign.AdCampaign) error
	StopAdCampaign(*adcampaign.AdCampaign) error

	StartAdSet(*adset.AdSet) error
	StopAdSet(*adset.AdSet) error

	StartAd(*ad.Ad) error
	StopAd(*ad.Ad) error

	Next(*adcampaign.AdCampaign, *adconfig.AdConfig, *adset.AdSet, *ad.Ad) error
}
