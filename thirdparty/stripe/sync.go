package stripe

import (
	"time"

	sgo "github.com/stripe/stripe-go/v84"

	"github.com/hanzoai/commerce/models/dispute"
	"github.com/hanzoai/commerce/models/paymentintent"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/refund"
	"github.com/hanzoai/commerce/models/setupintent"
	"github.com/hanzoai/commerce/models/types/currency"
)

// SyncPaymentIntent maps a Stripe PaymentIntent to a Commerce PaymentIntent.
func SyncPaymentIntent(dst *paymentintent.PaymentIntent, src *sgo.PaymentIntent) {
	if src.Customer != nil {
		dst.CustomerId = src.Customer.ID
	}
	dst.Amount = src.Amount
	dst.Currency = currency.Type(src.Currency)
	dst.Status = mapPIStatus(src.Status)
	if src.PaymentMethod != nil {
		dst.PaymentMethodId = src.PaymentMethod.ID
	}
	dst.CaptureMethod = string(src.CaptureMethod)
	dst.ConfirmationMethod = string(src.ConfirmationMethod)
	dst.AmountCapturable = src.AmountCapturable
	dst.AmountReceived = src.AmountReceived
	dst.Description = src.Description
	dst.ReceiptEmail = src.ReceiptEmail
	dst.ProviderRef = src.ID
	dst.ProviderType = "stripe"
	if src.CanceledAt != 0 {
		dst.CanceledAt = time.Unix(src.CanceledAt, 0)
	}
	dst.CancellationReason = string(src.CancellationReason)
	if src.LastPaymentError != nil {
		dst.LastError = src.LastPaymentError.Msg
	}
	dst.ClientSecret = src.ClientSecret
}

// SyncSetupIntent maps a Stripe SetupIntent to a Commerce SetupIntent.
func SyncSetupIntent(dst *setupintent.SetupIntent, src *sgo.SetupIntent) {
	if src.Customer != nil {
		dst.CustomerId = src.Customer.ID
	}
	if src.PaymentMethod != nil {
		dst.PaymentMethodId = src.PaymentMethod.ID
	}
	dst.Status = mapSIStatus(src.Status)
	dst.Usage = string(src.Usage)
	dst.ProviderRef = src.ID
	dst.ProviderType = "stripe"
	dst.CancellationReason = string(src.CancellationReason)
	if src.LastSetupError != nil {
		dst.LastError = src.LastSetupError.Msg
	}
	dst.ClientSecret = src.ClientSecret
}

// SyncPaymentMethod maps a Stripe PaymentMethod to a Commerce PaymentMethod.
func SyncPaymentMethod(dst *paymentmethod.PaymentMethod, src *sgo.PaymentMethod) {
	if src.Customer != nil {
		dst.CustomerId = src.Customer.ID
	}
	dst.Type = string(src.Type)
	dst.ProviderRef = src.ID
	dst.ProviderType = "stripe"

	if src.Card != nil {
		dst.Card = &paymentmethod.CardDetails{
			Brand:    string(src.Card.Brand),
			Last4:    src.Card.Last4,
			ExpMonth: int(src.Card.ExpMonth),
			ExpYear:  int(src.Card.ExpYear),
			Funding:  string(src.Card.Funding),
			Country:  src.Card.Country,
		}
	}

	if src.USBankAccount != nil {
		dst.BankAccount = &paymentmethod.BankAccountDetails{
			BankName:      src.USBankAccount.BankName,
			Last4:         src.USBankAccount.Last4,
			RoutingNumber: src.USBankAccount.RoutingNumber,
			AccountType:   string(src.USBankAccount.AccountHolderType),
		}
	}
}

// SyncRefund maps a Stripe Refund to a Commerce Refund.
func SyncRefund(dst *refund.Refund, src *sgo.Refund) {
	dst.Amount = src.Amount
	dst.Currency = currency.Type(src.Currency)
	dst.Status = mapRefundStatus(src.Status)
	dst.ProviderRef = src.ID
	dst.Reason = string(src.Reason)
	dst.ReceiptNumber = src.ReceiptNumber
	dst.FailureReason = string(src.FailureReason)
	if src.PaymentIntent != nil {
		dst.PaymentIntentId = src.PaymentIntent.ID
	}
}

// SyncDispute maps a Stripe Dispute to a Commerce Dispute.
func SyncDispute(dst *dispute.Dispute, src *sgo.Dispute) {
	dst.Amount = src.Amount
	dst.Currency = currency.Type(src.Currency)
	dst.Status = mapDisputeStatus(src.Status)
	dst.ProviderRef = src.ID
	dst.Reason = string(src.Reason)
	if src.EvidenceDetails != nil && src.EvidenceDetails.DueBy != 0 {
		dst.EvidenceDueBy = time.Unix(src.EvidenceDetails.DueBy, 0)
	}
	if src.PaymentIntent != nil {
		dst.PaymentIntentId = src.PaymentIntent.ID
	}
}

// MapWebhookEventType converts a Stripe webhook event type to a Commerce
// billing event type. Unknown types are returned as-is.
func MapWebhookEventType(stripeType string) string {
	switch stripeType {
	case "payment_intent.succeeded":
		return "payment_intent.succeeded"
	case "payment_intent.payment_failed":
		return "payment_intent.failed"
	case "payment_intent.canceled":
		return "payment_intent.canceled"
	case "payment_intent.created":
		return "payment_intent.created"
	case "payment_intent.requires_action":
		return "payment_intent.requires_action"
	case "setup_intent.succeeded":
		return "setup_intent.succeeded"
	case "setup_intent.setup_failed":
		return "setup_intent.failed"
	case "charge.refunded":
		return "refund.created"
	case "charge.dispute.created":
		return "dispute.created"
	case "charge.dispute.updated":
		return "dispute.updated"
	case "charge.dispute.closed":
		return "dispute.closed"
	case "customer.subscription.created":
		return "subscription.created"
	case "customer.subscription.updated":
		return "subscription.updated"
	case "customer.subscription.deleted":
		return "subscription.canceled"
	case "invoice.paid":
		return "invoice.paid"
	case "invoice.payment_failed":
		return "invoice.payment_failed"
	case "invoice.finalized":
		return "invoice.finalized"
	case "invoice.voided":
		return "invoice.voided"
	case "payment_method.attached":
		return "payment_method.attached"
	case "payment_method.detached":
		return "payment_method.detached"
	default:
		return stripeType
	}
}

// mapPIStatus converts a Stripe PaymentIntentStatus to a Commerce status.
func mapPIStatus(s sgo.PaymentIntentStatus) paymentintent.Status {
	switch s {
	case sgo.PaymentIntentStatusRequiresPaymentMethod:
		return paymentintent.RequiresPaymentMethod
	case sgo.PaymentIntentStatusRequiresConfirmation:
		return paymentintent.RequiresConfirmation
	case sgo.PaymentIntentStatusRequiresAction:
		return paymentintent.RequiresAction
	case sgo.PaymentIntentStatusProcessing:
		return paymentintent.Processing
	case sgo.PaymentIntentStatusRequiresCapture:
		return paymentintent.RequiresCapture
	case sgo.PaymentIntentStatusSucceeded:
		return paymentintent.Succeeded
	case sgo.PaymentIntentStatusCanceled:
		return paymentintent.Canceled
	default:
		return paymentintent.Status(s)
	}
}

// mapSIStatus converts a Stripe SetupIntentStatus to a Commerce status.
func mapSIStatus(s sgo.SetupIntentStatus) setupintent.Status {
	switch s {
	case sgo.SetupIntentStatusRequiresPaymentMethod:
		return setupintent.RequiresPaymentMethod
	case sgo.SetupIntentStatusRequiresConfirmation:
		return setupintent.RequiresConfirmation
	case sgo.SetupIntentStatusRequiresAction:
		return setupintent.RequiresAction
	case sgo.SetupIntentStatusProcessing:
		return setupintent.Processing
	case sgo.SetupIntentStatusSucceeded:
		return setupintent.Succeeded
	case sgo.SetupIntentStatusCanceled:
		return setupintent.Canceled
	default:
		return setupintent.Status(s)
	}
}

// mapRefundStatus converts a Stripe RefundStatus to a Commerce status.
func mapRefundStatus(s sgo.RefundStatus) refund.Status {
	switch s {
	case sgo.RefundStatusPending:
		return refund.Pending
	case sgo.RefundStatusSucceeded:
		return refund.Succeeded
	case sgo.RefundStatusFailed:
		return refund.Failed
	case sgo.RefundStatusCanceled:
		return refund.Canceled
	default:
		return refund.Status(s)
	}
}

// mapDisputeStatus converts a Stripe DisputeStatus to a Commerce status.
func mapDisputeStatus(s sgo.DisputeStatus) dispute.Status {
	switch s {
	case sgo.DisputeStatusWarningNeedsResponse:
		return dispute.WarningNeedsResponse
	case sgo.DisputeStatusNeedsResponse:
		return dispute.NeedsResponse
	case sgo.DisputeStatusUnderReview:
		return dispute.UnderReview
	case sgo.DisputeStatusWon:
		return dispute.Won
	case sgo.DisputeStatusLost:
		return dispute.Lost
	case sgo.DisputeStatusWarningUnderReview:
		return dispute.WarningUnderReview
	default:
		return dispute.Status(s)
	}
}
