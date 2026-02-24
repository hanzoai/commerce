package adconfig

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/orm"

	. "github.com/hanzoai/commerce/models/ads"
)

func init() { orm.Register[AdConfig]("adconfig") }

type AdConfig struct {
	mixin.Model[AdConfig]
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

func New(db *datastore.Datastore) *AdConfig {
	a := new(AdConfig)
	a.Init(db)
	return a
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("adconfig")
}
