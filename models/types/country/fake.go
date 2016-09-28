package country

import "math/rand"

var numCountries = len(Countries)

func Fake() Country {
	return Countries[rand.Intn(numCountries)]
}
