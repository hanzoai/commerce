package promotionrule

import "github.com/hanzoai/commerce/datastore"

var kind = "promotionrule"

func (p PromotionRule) Kind() string {
	return kind
}

func (p *PromotionRule) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func (p *PromotionRule) Defaults() {
}

func New(db *datastore.Datastore) *PromotionRule {
	p := new(PromotionRule)
	p.Init(db)
	p.Defaults()
	return p
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
