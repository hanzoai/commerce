package currency

import "strconv"

type Cents int

func CentsFromString(s string) Cents {
	f, _ := strconv.ParseFloat(s, 64)
	return Cents(int(f * 100))
}
