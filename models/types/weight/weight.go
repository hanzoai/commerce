package weight

type Mass float64
type Unit string

const (
	Pound    Unit = "lb"
	Ounce         = "oz"
	Kilogram      = "kg"
	Gram          = "g"
)

// Convert everything to grams
var conversions = map[Unit]float64{Pound: 453.592, Ounce: 28.3495, Kilogram: 1000, Gram: 1}

func Convert(mass Mass, from, to Unit) Mass {
	fromG := conversions[from]
	toG := 1 / conversions[to]

	// Convert to and then from grams
	return Mass(float64(mass) * toG * fromG)
}

var Units = []Unit{Pound, Ounce, Kilogram, Gram}
