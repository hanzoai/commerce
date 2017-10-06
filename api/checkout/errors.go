package checkout

import "errors"

var (
	FailedToCreateCustomer       = errors.New("Failed to create customer")
	FailedToCreateUser           = errors.New("Failed to create user")
	FailedToDecodeRequestBody    = errors.New("Failed to decode request body")
	FeeCalculationError          = errors.New("Failed to calculate fees")
	FundingAccountCreationError  = errors.New("Failed to create funding account")
	InvalidOrIncompleteOrder     = errors.New("Invalid or incomplete order")
	OnlyOneOfUserBuyerAllowed    = errors.New("Only one of user buyer allowed")
	OrderDoesNotExist            = errors.New("Order does not exist")
	PaymentCancelled             = errors.New("Payment was cancelled")
	TokenSaleNotFound            = errors.New("Token sale not found")
	UnsupportedEthereumCurrency  = errors.New("Only ETH is supported for 'ethereum' payment method")
	UnsupportedPaymentType       = errors.New("Unsupported payment type")
	UnsupportedStripeCurrency    = errors.New("XBT(BTC), ETH not supported by 'stripe' payment method")
	UserDoesNotExist             = errors.New("User does not exist")
	WalletCreationError          = errors.New("Failed to create wallet for user")
	MissingTokenSaleOrPassphrase = errors.New("order.tokenSaleId or tokenSale.passphrase is missing")
)
