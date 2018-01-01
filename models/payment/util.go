package payment

// Is the payment processor type handle fiat
func IsFiatProcessorType(typ Type) bool {
	switch typ {
	case Bitcoin:
		return false
	case Ethereum:
		return false
	default:
		return true
	}
}
