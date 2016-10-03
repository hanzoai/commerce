package country

import "math/rand"

func Fake() Country {
	return Countries[rand.Intn(numCountries)]
}
