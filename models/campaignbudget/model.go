package campaignbudget

import "github.com/hanzoai/commerce/datastore"

var kind = "campaignbudget"

func (c CampaignBudget) Kind() string {
	return kind
}

func (c *CampaignBudget) Init(db *datastore.Datastore) {
	c.Model.Init(db, c)
}

func (c *CampaignBudget) Defaults() {
}

func New(db *datastore.Datastore) *CampaignBudget {
	c := new(CampaignBudget)
	c.Init(db)
	c.Defaults()
	return c
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
