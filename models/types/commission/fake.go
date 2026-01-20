package commission

import (
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/fake"
)

func Fake() Commission {
	var c Commission
	c.Flat = currency.Cents(0).Fake()
	c.Minimum = currency.Cents(0).Fake()
	c.Percent = fake.Percent
	return c
}
