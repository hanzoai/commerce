package errors

import (
	"errors"
)

var (
	FailedToCreateCustomer          = errors.New("Failed to create Authorize customer.")
	FailedToUpdateCustomer          = errors.New("Failed to update Authorize customer.")
	RefundGreaterThanPayment        = errors.New("The requested refund amount is greater than the paid amount")
	UnableToRefundUnpaidTransaction = errors.New("Unable to refund unpaid transaction")
)

type AuthorizeError struct {
	Type    string
	Message string
	Code    string
	Param   string
}

func (e AuthorizeError) Error() string {
	return e.Message
}

func New(err error) error {
	return err
}
