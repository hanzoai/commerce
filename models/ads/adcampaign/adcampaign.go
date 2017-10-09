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

func (a AdCampaign) GetAdConfigSearchFieldAndIds() (string, []string) {
	return "AdCampaignId", []string{a.Id()}
}

func (a AdCampaign) GetAdSetSearchFieldAndIds() (string, []string) {
	return "AdCampaignId", []string{a.Id()}
}

func (a AdCampaign) GetAdSearchFieldAndIds() (string, []string) {
	return "AdCampaignId", []string{a.Id()}
}

func (a AdCampaign) GetHeadlineSearchFieldAndIds() (string, []string) {
	return "AdCampaignId", []string{a.Id()}
}

func (a AdCampaign) GetCopySearchFieldAndIds() (string, []string) {
	return "AdCampaignId", []string{a.Id()}
}

func (a AdCampaign) GetMediaSearchFieldAndIds() (string, []string) {
	return "AdCampaignId", []string{a.Id()}
}
