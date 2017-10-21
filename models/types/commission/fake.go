package commission

import (
	"hanzo.io/models/types/currency"
	"hanzo.io/util/fake"
)

func Fake() Commission {
	var c Commission
	c.Flat = currency.NewCents(0).Fake()
	c.Minimum = currency.NewCents(0).Fake()
	c.Percent = fake.Percent
	return c
}
