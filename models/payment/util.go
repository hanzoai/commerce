package payment

import "hanzo.io/models/types/accounts"

// Is the payment processor type handle fiat
func IsFiatProcessorType(typ accounts.Type) bool {
	switch typ {
	case accounts.BitcoinType:
		return false
	case accounts.EthereumType:
		return false
	default:
		return true
	}
}
