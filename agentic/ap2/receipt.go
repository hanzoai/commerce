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
// (checkout_receipt.json). Result repeats Status as the general Mandate Receipt names it
// (agent_authorization.md), which requires `result` where this schema requires `status`.
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

// PaymentReceipt is the signed record of what became of a Payment Mandate (payment_receipt.json).
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

// receipt is what both receipts are to the code that signs and reads them.
type receipt interface {
	finish() error // sets Result, then validates
	valid() error
}

func (r *CheckoutReceipt) finish() error { r.Result = result(r.Status); return r.valid() }
func (r *PaymentReceipt) finish() error  { r.Result = result(r.Status); return r.valid() }

func result(s Status) string {
	if s == Success {
		return "success"
	}
	return "error"
}

// valid reports whether the receipt is one of the two shapes the schema allows: a success naming
// its order and carrying no error, or an error carrying both error members and no order.
func (r *CheckoutReceipt) valid() error {
	switch {
	case r.Issuer == "" || r.Reference == "" || r.IssuedAt == 0 || r.Result != result(r.Status):
		return errors.New("a receipt needs iss, iat, reference and the result of its status")
	case r.Status == Success && (r.OrderID == "" || r.Error != "" || r.ErrorDescription != ""):
		return errors.New("a success receipt names its order and has no error")
	case r.Status == Failure && (r.Error == "" || r.ErrorDescription == "" || r.OrderID != ""):
		return errors.New("an error receipt has error and error_description and no order")
	case r.Status != Success && r.Status != Failure:
		return fmt.Errorf("status %q", r.Status)
	}
	return nil
}

// valid reports whether the receipt is one of the two shapes the schema allows: a success carrying
// both confirmation ids and no error, or an error carrying both error members and no confirmation.
func (r *PaymentReceipt) valid() error {
	switch {
	case r.Issuer == "" || r.Reference == "" || r.IssuedAt == 0 || r.PaymentID == "" || r.Result != result(r.Status):
		return errors.New("a receipt needs iss, iat, reference, payment_id and the result of its status")
	case r.Status == Success && (r.PSPConfirmationID == "" || r.NetworkConfirmationID == "" || r.Error != "" || r.ErrorDescription != ""):
		return errors.New("a success receipt carries psp_confirmation_id and network_confirmation_id and has no error")
	case r.Status == Failure && (r.Error == "" || r.ErrorDescription == "" || r.PSPConfirmationID != "" || r.NetworkConfirmationID != ""):
		return errors.New("an error receipt has error and error_description and no confirmation")
	case r.Status != Success && r.Status != Failure:
		return fmt.Errorf("status %q", r.Status)
	}
	return nil
}

// receipt signs a receipt as a compact JWT.
func (s Signer) receipt(r receipt) (string, error) {
	if err := r.finish(); err != nil {
		return "", fmt.Errorf("ap2: %w", err)
	}
	return s.Sign(r)
}

// ParseCheckoutReceipt verifies a Checkout Receipt under the issuer's key. When mandate, the
// presentation of the closed Checkout Mandate, is given, the receipt's `reference` must be the
// hash of its final SD-JWT (specification.md, "Dispute").
func ParseCheckoutReceipt(token string, issuer *ecdsa.PublicKey, mandate string) (*CheckoutReceipt, error) {
	var r CheckoutReceipt
	if err := parseReceipt(token, issuer, mandate, &r, &r.Reference); err != nil {
		return nil, err
	}
	return &r, nil
}

// ParsePaymentReceipt is ParseCheckoutReceipt for a Payment Receipt and a Payment Mandate.
func ParsePaymentReceipt(token string, issuer *ecdsa.PublicKey, mandate string) (*PaymentReceipt, error) {
	var r PaymentReceipt
	if err := parseReceipt(token, issuer, mandate, &r, &r.Reference); err != nil {
		return nil, err
	}
	return &r, nil
}

func parseReceipt(token string, issuer *ecdsa.PublicKey, mandate string, into receipt, reference *string) error {
	j, err := parseJWS(token)
	if err == nil {
		err = j.verify(issuer)
	}
	if err != nil {
		return err
	}
	if _, err := object(j.payload); err != nil {
		return fmt.Errorf("%w: receipt: %v", ErrMalformed, err)
	}
	if err := json.Unmarshal(j.payload, into); err != nil {
		return fmt.Errorf("%w: receipt: %v", ErrMalformed, err)
	}
	if err := into.valid(); err != nil {
		return fmt.Errorf("%w: receipt: %v", ErrMalformed, err)
	}
	if mandate == "" {
		return nil
	}
	p, perr := parsePresentation(mandate)
	if perr != nil {
		return perr
	}
	if *reference != p.reference() {
		return fmt.Errorf("%w: the receipt's reference is not the hash of the mandate", ErrBinding)
	}
	return nil
}
