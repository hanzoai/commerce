package adcampaign

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/orm"

	. "github.com/hanzoai/commerce/models/ads"
)

func init() { orm.Register[AdCampaign]("adcampaign") }

type Engine string

const (
	DemoEngine Engine = "demo"
)

type FacebookAdCampaign struct {
}

type AdCampaign struct {
	mixin.Model[AdCampaign]
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

func New(db *datastore.Datastore) *AdCampaign {
	a := new(AdCampaign)
	a.Init(db)
	a.Status = PendingStatus
	return a
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("adcampaign")
}
