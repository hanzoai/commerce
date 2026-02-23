package stripe

import (
	"testing"
	"time"

	sgo "github.com/stripe/stripe-go/v84"

	"github.com/hanzoai/commerce/models/transfer"
	"github.com/hanzoai/commerce/models/types/currency"
)

func TestUpdateTransferFromStripe_Paid(t *testing.T) {
	str := &Transfer{
		Amount:         5000,
		AmountReversed: 100,
		Currency:       "usd",
		Livemode:       true,
		Reversed:       false,
		Description:    "Transfer desc",
		Destination:    &sgo.Account{ID: "acct_dest", Type: "custom"},
		DestinationPayment: &sgo.Charge{
			ApplicationFeeAmount: 250,
			Status:               "paid",
		},
		SourceTransaction: &sgo.Charge{ID: "txn_src"},
		SourceType:        sgo.TransferSourceTypeCard,
	}

	tr := &transfer.Transfer{}
	UpdateTransferFromStripe(tr, str)

	if tr.Amount != currency.Cents(5000) {
		t.Errorf("Amount = %d, want 5000", tr.Amount)
	}
	if tr.AmountReversed != currency.Cents(100) {
		t.Errorf("AmountReversed = %d, want 100", tr.AmountReversed)
	}
	if tr.Currency != currency.Type("usd") {
		t.Errorf("Currency = %q, want %q", tr.Currency, "usd")
	}
	if !tr.Live {
		t.Error("Live = false, want true")
	}
	if tr.Account.ApplicationFee != 250 {
		t.Errorf("Account.ApplicationFee = %d, want 250", tr.Account.ApplicationFee)
	}
	if tr.Account.BalanceTransaction != 5000 {
		t.Errorf("Account.BalanceTransaction = %d, want 5000", tr.Account.BalanceTransaction)
	}
	if tr.Account.Destination != "acct_dest" {
		t.Errorf("Account.Destination = %q, want %q", tr.Account.Destination, "acct_dest")
	}
	if tr.Account.DestinationType != "custom" {
		t.Errorf("Account.DestinationType = %q, want %q", tr.Account.DestinationType, "custom")
	}
	if tr.Account.Reversed {
		t.Error("Account.Reversed = true, want false")
	}
	if tr.Account.SourceTransaction != "txn_src" {
		t.Errorf("Account.SourceTransaction = %q, want %q", tr.Account.SourceTransaction, "txn_src")
	}
	if tr.Account.SourceType != "card" {
		t.Errorf("Account.SourceType = %q, want %q", tr.Account.SourceType, "card")
	}
	if tr.Account.StatementDescriptor != "Transfer desc" {
		t.Errorf("Account.StatementDescriptor = %q, want %q", tr.Account.StatementDescriptor, "Transfer desc")
	}
	if tr.Account.Type != "paid" {
		t.Errorf("Account.Type = %q, want %q", tr.Account.Type, "paid")
	}
	if tr.Status != transfer.Paid {
		t.Errorf("Status = %q, want %q", tr.Status, transfer.Paid)
	}
}

func TestUpdateTransferFromStripe_StatusMapping(t *testing.T) {
	cases := []struct {
		stripeStatus sgo.ChargeStatus
		want         transfer.Status
	}{
		{"paid", transfer.Paid},
		{"pending", transfer.Pending},
		{"in_transit", transfer.InTransit},
		{"cancelled", transfer.Canceled},
		{"failed", transfer.Failed},
	}

	for _, tc := range cases {
		t.Run(string(tc.stripeStatus), func(t *testing.T) {
			str := &Transfer{
				Destination:        &sgo.Account{},
				DestinationPayment: &sgo.Charge{Status: tc.stripeStatus},
				SourceTransaction:  &sgo.Charge{},
			}
			tr := &transfer.Transfer{}
			UpdateTransferFromStripe(tr, str)
			if tr.Status != tc.want {
				t.Errorf("Status = %q, want %q", tr.Status, tc.want)
			}
		})
	}
}

func TestUpdateTransferFromStripe_UnknownStatus(t *testing.T) {
	str := &Transfer{
		Destination:        &sgo.Account{},
		DestinationPayment: &sgo.Charge{Status: "exotic"},
		SourceTransaction:  &sgo.Charge{},
	}
	tr := &transfer.Transfer{}
	UpdateTransferFromStripe(tr, str)

	// Unknown status does not match any case, Status remains zero value.
	if tr.Status != "" {
		t.Errorf("Status = %q, want empty (unknown status)", tr.Status)
	}
}

func TestUpdatePayoutFromStripe_Paid(t *testing.T) {
	str := &Payout{
		Amount:              3000,
		Currency:            "eur",
		Livemode:            true,
		ArrivalDate:         1700000000,
		Created:             1699999000,
		StatementDescriptor: "Payout desc",
		Destination:         &sgo.PayoutDestination{ID: "ba_dest", Type: "bank_account"},
		FailureCode:         sgo.PayoutFailureCodeAccountClosed,
		FailureMessage:      "Account closed",
		SourceType:          sgo.PayoutSourceTypeCard,
		Type:                sgo.PayoutTypeCard,
		Status:              "paid",
	}

	tr := &transfer.Transfer{}
	UpdatePayoutFromStripe(tr, str)

	if tr.Amount != currency.Cents(3000) {
		t.Errorf("Amount = %d, want 3000", tr.Amount)
	}
	if tr.Currency != currency.Type("eur") {
		t.Errorf("Currency = %q, want %q", tr.Currency, "eur")
	}
	if !tr.Live {
		t.Error("Live = false, want true")
	}
	wantDate := time.Unix(1700000000, 0)
	if !tr.Account.Date.Equal(wantDate) {
		t.Errorf("Account.Date = %v, want %v", tr.Account.Date, wantDate)
	}
	wantCreated := time.Unix(1699999000, 0)
	if !tr.Account.Created.Equal(wantCreated) {
		t.Errorf("Account.Created = %v, want %v", tr.Account.Created, wantCreated)
	}
	if tr.Account.Description != "Payout desc" {
		t.Errorf("Account.Description = %q, want %q", tr.Account.Description, "Payout desc")
	}
	if tr.Account.Destination != "ba_dest" {
		t.Errorf("Account.Destination = %q, want %q", tr.Account.Destination, "ba_dest")
	}
	if tr.Account.DestinationType != "bank_account" {
		t.Errorf("Account.DestinationType = %q, want %q", tr.Account.DestinationType, "bank_account")
	}
	if tr.Account.FailureCode != "account_closed" {
		t.Errorf("Account.FailureCode = %q, want %q", tr.Account.FailureCode, "account_closed")
	}
	if tr.Account.FailureMessage != "Account closed" {
		t.Errorf("Account.FailureMessage = %q, want %q", tr.Account.FailureMessage, "Account closed")
	}
	if tr.Account.SourceType != "card" {
		t.Errorf("Account.SourceType = %q, want %q", tr.Account.SourceType, "card")
	}
	if tr.Account.StatementDescriptor != "Payout desc" {
		t.Errorf("Account.StatementDescriptor = %q, want %q", tr.Account.StatementDescriptor, "Payout desc")
	}
	if tr.Account.Type != "card" {
		t.Errorf("Account.Type = %q, want %q", tr.Account.Type, "card")
	}
	if tr.Status != transfer.Paid {
		t.Errorf("Status = %q, want %q", tr.Status, transfer.Paid)
	}
}

func TestUpdatePayoutFromStripe_StatusMapping(t *testing.T) {
	cases := []struct {
		stripeStatus sgo.PayoutStatus
		want         transfer.Status
	}{
		{"paid", transfer.Paid},
		{"pending", transfer.Pending},
		{"in_transit", transfer.InTransit},
		{"cancelled", transfer.Canceled},
		{"failed", transfer.Failed},
	}

	for _, tc := range cases {
		t.Run(string(tc.stripeStatus), func(t *testing.T) {
			str := &Payout{
				Status:      tc.stripeStatus,
				Destination: &sgo.PayoutDestination{},
			}
			tr := &transfer.Transfer{}
			UpdatePayoutFromStripe(tr, str)
			if tr.Status != tc.want {
				t.Errorf("Status = %q, want %q", tr.Status, tc.want)
			}
		})
	}
}

func TestUpdatePayoutFromStripe_UnknownStatus(t *testing.T) {
	str := &Payout{
		Status:      "exotic",
		Destination: &sgo.PayoutDestination{},
	}
	tr := &transfer.Transfer{}
	UpdatePayoutFromStripe(tr, str)

	if tr.Status != "" {
		t.Errorf("Status = %q, want empty (unknown status)", tr.Status)
	}
}

func TestUpdatePayoutFromStripe_ZeroValues(t *testing.T) {
	// Destination must be non-nil (code dereferences without nil check).
	str := &Payout{
		Destination: &sgo.PayoutDestination{},
	}
	tr := &transfer.Transfer{}
	UpdatePayoutFromStripe(tr, str)

	if tr.Amount != 0 {
		t.Errorf("Amount = %d, want 0", tr.Amount)
	}
	if tr.Currency != "" {
		t.Errorf("Currency = %q, want empty", tr.Currency)
	}
	if tr.Live {
		t.Error("Live = true, want false")
	}
	// ArrivalDate=0 and Created=0 produce Unix epoch
	epoch := time.Unix(0, 0)
	if !tr.Account.Date.Equal(epoch) {
		t.Errorf("Account.Date = %v, want epoch", tr.Account.Date)
	}
	if !tr.Account.Created.Equal(epoch) {
		t.Errorf("Account.Created = %v, want epoch", tr.Account.Created)
	}
}
