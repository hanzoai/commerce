package adconfig

import (
	"github.com/hanzoai/commerce/models/mixin"

	. "github.com/hanzoai/commerce/models/ads"
)

type AdConfig struct {
	mixin.Model
	FacebookAdTypePlacements

	AdCampaignId string `json:"adCampaignId"`
}

func (a AdConfig) GetAdCampaignId() string {
	return a.AdCampaignId
}

func (a AdConfig) GetAdSetSearchFieldAndIds() (string, []string) {
	return "AdConfigId", []string{a.Id()}
}

func (a AdConfig) GetAdSearchFieldAndIds() (string, []string) {
	return "AdConfigId", []string{a.Id()}
}

func (a AdConfig) GetHeadlineSearchFieldAndIds() (string, []string) {
	return "AdConfigId", []string{a.Id()}
}

func (a AdConfig) GetCopySearchFieldAndIds() (string, []string) {
	return "AdConfigId", []string{a.Id()}
}

func (a AdConfig) GetMediaSearchFieldAndIds() (string, []string) {
	return "AdConfigId", []string{a.Id()}
}
