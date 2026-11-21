// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
)

// Status is the `status` of a receipt (types/receipt_status.json).
type Status string

// The two statuses of a receipt.
const (
	Success Status = "Success"
	Failure Status = "Error"
)

// CheckoutReceipt is the signed record of what became of a Checkout Mandate
// (checkout_receipt.json). Result repeats Status in the vocabulary of the general Mandate
// Receipt (agent_authorization.md), which requires `result` where the checkout schema
// requires `status`.
type CheckoutReceipt struct {
	Status           Status `json:"status"`
	Result           string `json:"result"`
	Issuer           string `json:"iss"`
	IssuedAt         int64  `json:"iat"`
	Reference        string `json:"reference"`
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
	OrderID          string `json:"order_id,omitempty"`
}

// PaymentReceipt is the signed record of what became of a Payment Mandate
// (payment_receipt.json).
type PaymentReceipt struct {
	Status                Status `json:"status"`
	Result                string `json:"result"`
	Issuer                string `json:"iss"`
	IssuedAt              int64  `json:"iat"`
	Reference             string `json:"reference"`
	Error                 string `json:"error,omitempty"`
	ErrorDescription      string `json:"error_description,omitempty"`
	PaymentID             string `json:"payment_id"`
	PSPConfirmationID     string `json:"psp_confirmation_id,omitempty"`
	NetworkConfirmationID string `json:"network_confirmation_id,omitempty"`
}

// result is the `result` that goes with a status.
func result(s Status) string {
	if s == Success {
		return "success"
	}
	return "error"
}

// Valid reports whether the receipt is one of the two shapes the schema allows: a success
// that names its order and carries no error, or an error that carries both error members
// and no order.
func (r CheckoutReceipt) Valid() error {
	switch {
	case r.Issuer == "" || r.Reference == "" || r.IssuedAt == 0:
		return errors.New("a receipt needs iss, iat and reference")
	case r.Status == Success && (r.OrderID == "" || r.Error != "" || r.ErrorDescription != ""):
		return errors.New("a success receipt names its order and has no error")
	case r.Status == Failure && (r.Error == "" || r.ErrorDescription == "" || r.OrderID != ""):
		return errors.New("an error receipt has error and error_description and no order")
	case r.Status != Success && r.Status != Failure:
		return fmt.Errorf("status %q", r.Status)
	}
	return nil
}

// Valid reports whether the receipt is one of the two shapes the schema allows: a success
// that carries both confirmation ids and no error, or an error that carries both error
// members and no confirmation.
func (r PaymentReceipt) Valid() error {
	switch {
	case r.Issuer == "" || r.Reference == "" || r.IssuedAt == 0 || r.PaymentID == "":
		return errors.New("a receipt needs iss, iat, reference and payment_id")
	case r.Status == Success && (r.PSPConfirmationID == "" || r.NetworkConfirmationID == "" || r.Error != "" || r.ErrorDescription != ""):
		return errors.New("a success receipt carries psp_confirmation_id and network_confirmation_id and has no error")
	case r.Status == Failure && (r.Error == "" || r.ErrorDescription == "" || r.PSPConfirmationID != "" || r.NetworkConfirmationID != ""):
		return errors.New("an error receipt has error and error_description and no confirmation")
	case r.Status != Success && r.Status != Failure:
		return fmt.Errorf("status %q", r.Status)
	}
	return nil
}

// SignCheckoutReceipt signs the receipt as a compact JWT.
func SignCheckoutReceipt(s Signer, r CheckoutReceipt) (string, error) {
	r.Result = result(r.Status)
	if err := r.Valid(); err != nil {
		return "", fmt.Errorf("ap2: %w", err)
	}
	return s.Sign(r)
}

// SignPaymentReceipt signs the receipt as a compact JWT.
func SignPaymentReceipt(s Signer, r PaymentReceipt) (string, error) {
	r.Result = result(r.Status)
	if err := r.Valid(); err != nil {
		return "", fmt.Errorf("ap2: %w", err)
	}
	return s.Sign(r)
}

// ParseCheckoutReceipt verifies a Checkout Receipt under the issuer's key. When mandate is
// not empty, the presentation of the closed Checkout Mandate the receipt answers, the
// receipt's `reference` must be the hash of that mandate's final SD-JWT (specification.md,
// "Dispute").
func ParseCheckoutReceipt(receipt string, issuer *ecdsa.PublicKey, mandate string) (*CheckoutReceipt, error) {
	var r CheckoutReceipt
	if err := parseReceipt(receipt, issuer, mandate, &r, func() error { return r.Valid() }); err != nil {
		return nil, err
	}
	return &r, nil
}

// ParsePaymentReceipt verifies a Payment Receipt under the issuer's key, and against the
// presentation of the closed Payment Mandate when one is given.
func ParsePaymentReceipt(receipt string, issuer *ecdsa.PublicKey, mandate string) (*PaymentReceipt, error) {
	var r PaymentReceipt
	if err := parseReceipt(receipt, issuer, mandate, &r, func() error { return r.Valid() }); err != nil {
		return nil, err
	}
	return &r, nil
}

func parseReceipt(receipt string, issuer *ecdsa.PublicKey, mandate string, into any, valid func() error) error {
	j, err := parseJWS(receipt)
	if err != nil {
		return err
	}
	if err := j.verify(issuer); err != nil {
		return err
	}
	if _, err := object(j.payload); err != nil {
		return fmt.Errorf("%w: receipt: %v", ErrMalformed, err)
	}
	if err := json.Unmarshal(j.payload, into); err != nil {
		return fmt.Errorf("%w: receipt: %v", ErrMalformed, err)
	}
	if err := valid(); err != nil {
		return fmt.Errorf("%w: receipt: %v", ErrMalformed, err)
	}
	if mandate == "" {
		return nil
	}
	p, perr := parsePresentation(mandate)
	if perr != nil {
		return perr
	}
	ref, err := p.reference()
	if err != nil {
		return err
	}
	var got string
	switch r := into.(type) {
	case *CheckoutReceipt:
		got = r.Reference
	case *PaymentReceipt:
		got = r.Reference
	}
	if got != ref {
		return fmt.Errorf("%w: the receipt's reference is not the hash of the mandate", ErrBinding)
	}
	return nil
}
