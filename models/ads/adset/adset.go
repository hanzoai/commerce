package adset

import (
	"github.com/hanzoai/commerce/models/mixin"

	. "github.com/hanzoai/commerce/models/ads"
)

type FacebookAdSet struct {
}

type AdSet struct {
	mixin.BaseModel
	FacebookAdSet

	AdCampaignId string `json:"adCampaignId"`
	AdConfigId   string `json:"adConfigId"`

	Status Status `json:"status"`
}

func (a AdSet) GetAdCampaignId() string {
	return a.AdCampaignId
}

func (a AdSet) GetAdConfigId() string {
	return a.AdConfigId
}

func (a AdSet) GetAdSearchFieldAndIds() (string, []string) {
	return "AdSetId", []string{a.Id()}
}

func (a AdSet) GetHeadlineSearchFieldAndIds() (string, []string) {
	return "AdSetId", []string{a.Id()}
}

func (a AdSet) GetCopySearchFieldAndIds() (string, []string) {
	return "AdSetId", []string{a.Id()}
}

func (a AdSet) GetMediaSearchFieldAndIds() (string, []string) {
	return "AdSetId", []string{a.Id()}
}
