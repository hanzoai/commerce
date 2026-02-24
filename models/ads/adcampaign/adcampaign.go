package adcampaign

import (
	"github.com/hanzoai/commerce/models/mixin"

	. "github.com/hanzoai/commerce/models/ads"
)

type Engine string

const (
	DemoEngine Engine = "demo"
)

type FacebookAdCampaign struct {
}

type AdCampaign struct {
	mixin.BaseModel
	FacebookAdCampaign
	StatsWeCareAbout

	Name   string `json:"name"`
	Engine Engine `json:"engine"`
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
