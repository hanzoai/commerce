package adset

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/orm"

	. "github.com/hanzoai/commerce/models/ads"
)

func init() { orm.Register[AdSet]("adset") }

type FacebookAdSet struct {
}

type AdSet struct {
	mixin.Model[AdSet]
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

func New(db *datastore.Datastore) *AdSet {
	a := new(AdSet)
	a.Init(db)
	a.Status = PendingStatus
	return a
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("adset")
}
