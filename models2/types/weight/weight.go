package weight

type Unit string

const (
	Pound    Unit = "lb"
	Ounce         = "oz"
	Kilogram      = "kg"
	Gram          = "g"
)

type Weight struct {
	Mass float64
	Unit Unit
}

// Convert everything to grams
var conversions = map[Unit]float64{Pound: 453.592, Ounce: 28.3495, Kilogram: 1000, Gram: 1}

func (w Weight) convert(u Unit) Weight {
	toG := conversions[w.Unit]
	fromG := 1 / conversions[u]

	// Convert to and then from grams
	return Weight{Mass: w.Mass * toG * fromG, Unit: u}
}
