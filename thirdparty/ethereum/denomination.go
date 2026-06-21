package ethereum

// Ether denomination multipliers, expressed in wei.
//
// Wei is the base, indivisible unit of ether. The names below are the
// standard community labels for common powers-of-ten of wei; the values are
// plain decimal unit-conversion facts.
//
//	1 Shannon (Gwei) = 1e9 wei
//
// Use these as scalar multipliers, e.g.
//
//	gasPrice := new(big.Int).Mul(big.NewInt(gwei), big.NewInt(Shannon))
const (
	Wei     = 1
	Shannon = 1e9 // a.k.a. Gwei, the customary unit for gas prices
	Ether   = 1e18
)
