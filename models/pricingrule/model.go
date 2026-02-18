package pricingrule

import "github.com/hanzoai/commerce/datastore"

var kind = "billing-pricing-rule"

func (p PricingRule) Kind() string {
	return kind
}

func (p *PricingRule) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func (p *PricingRule) Defaults() {
	p.Parent = p.Db.NewKey("synckey", "", 1, nil)
	if p.Currency == "" {
		p.Currency = "usd"
	}
	if p.PricingType == "" {
		p.PricingType = PerUnit
	}
}

func New(db *datastore.Datastore) *PricingRule {
	p := new(PricingRule)
	p.Init(db)
	p.Defaults()
	return p
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
