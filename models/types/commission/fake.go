package commission

import (
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/fake"
)

func Fake() Commission {
	var c Commission
	c.Flat = currency.Cents(0).Fake()
	c.Minimum = currency.Cents(0).Fake()
	c.Percent = fake.Percent
	return c
}
