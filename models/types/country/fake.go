package country

import (
	"math/rand"
	"strings"
)

func Fake() string {
	all := All()
	return strings.ToLower(all[rand.Intn(len(all))].Codes.Alpha2)
}
