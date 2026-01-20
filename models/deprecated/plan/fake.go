package plan

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/fake"
	"github.com/hanzoai/commerce/models/types/currency"

	. "github.com/hanzoai/commerce/types"
)

func Fake(db *datastore.Datastore) *Plan {
	p := New(db)
	p.Slug = fake.Slug()
	p.SKU = fake.SKU()
	p.Name = fake.Word()
	p.Id_ = fake.Id()
	p.Description = fake.Word()
	p.Price = currency.Cents(0).Fake()
	p.Currency = currency.USD
	p.Interval = Monthly
	return p
}
