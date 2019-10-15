package account

import (
	"errors"
)

var ErrorAccountNotWithdrawable = errors.New("Account not withdrawable.")
var ErrorInsufficientFunds = errors.New("Source has insufficient funds")
var ErrorInvalidPaymentMethod = errors.New("Invalid payment method")
