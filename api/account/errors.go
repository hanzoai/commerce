package account

import (
	"errors"
)

var ErrorAccountNotWithdrawable = errors.New("Account not withdrawable.")
var ErrorInsufficientFunds = errors.New("Source has insufficient funds")
