package currency

import (
	"hanzo.io/util/betterbig"
	"strconv"
)

type Cents struct {
	betterbig.Int
}

var NewInt = betterbig.NewInt

func NewCents(z int64) Cents {
	return Cents{betterbig.NewInt(z)}
}

func CentsFromString(s string) Cents {
	f, _ := strconv.ParseFloat(s, 64)
	return NewCents(int64(f * 100))
}
