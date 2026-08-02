package currency

// Cents is an amount in a currency's minor units — despite the name it is
// minor units, not cents: for a zero-decimal currency (see Type.IsZeroDecimal)
// one Cents is one whole unit.
//
// There is deliberately no parser here. Converting a decimal string of MAJOR
// units ("19.99") into minor units is one fact, and its home is
// money.ParseCents in github.com/hanzoai/money — which commerce already
// depends on. Call that and convert the int64 it returns.
type Cents int64
