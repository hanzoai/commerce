package weight

import "strconv"

const (
	Pound    Unit = "lb"
	Ounce    Unit = "oz"
	Kilogram Unit = "kg"
	Gram     Unit = "g"
)

type Mass float64

func (m Mass) String() string {
	return strconv.FormatFloat(float64(m), 'f', 2, 64)
}

type Unit string

// Print nice names
func (u Unit) Name() string {
	switch u {
	case "lb":
		return "Pound"
	case "oz":
		return "Ounce"
	case "kg":
		return "Kilogram"
	case "g":
		return "Gram"
	}
	return ""
}

// Convert everything to grams
var conversions = map[Unit]float64{Pound: 453.592, Ounce: 28.3495, Kilogram: 1000, Gram: 1}

func Convert(mass Mass, from, to Unit) Mass {
	fromG := conversions[from]
	toG := 1 / conversions[to]

	// Convert to and then from grams
	return Mass(float64(mass) * toG * fromG)
}

var Units = []Unit{Pound, Ounce, Kilogram, Gram}
