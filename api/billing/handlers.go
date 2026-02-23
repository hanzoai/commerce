package billing

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/router"
)

// Route registers billing endpoints for service-to-service calls.
// These are internal endpoints used by Cloud-API; require admin token.
func Route(r router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := r.Group("billing")
	api.Use(adminRequired)

	// Balance & usage (existing)
	api.GET("/balance", GetBalance)
	api.GET("/balance/all", GetBalanceAll)
	api.GET("/usage", GetUsage)
	api.POST("/usage", RecordUsage)
	api.POST("/deposit", Deposit)
	api.POST("/credit", GrantStarterCredit)
	api.POST("/refund", Refund)

	// Meters
	api.POST("/meters", CreateMeter)
	api.GET("/meters", ListMeters)
	api.GET("/meters/:id", GetMeter)

	// Meter events
	api.POST("/meter-events", RecordMeterEvents)
	api.GET("/meter-events/summary", GetMeterEventsSummary)

	// Credit grants
	api.POST("/credit-grants", CreateCreditGrant)
	api.GET("/credit-grants", ListCreditGrants)
	api.GET("/credit-balance", GetCreditBalance)
	api.POST("/credit-grants/:id/void", VoidCreditGrant)

	// Pricing rules
	api.POST("/pricing-rules", CreatePricingRule)
	api.GET("/pricing-rules", ListPricingRules)
	api.DELETE("/pricing-rules/:id", DeletePricingRule)

	// Invoice preview (legacy)
	api.POST("/invoice-preview", InvoicePreview)

	// Billing invoices
	api.POST("/invoices", CreateInvoice)
	api.GET("/invoices", ListInvoices)
	api.GET("/invoices/upcoming", UpcomingInvoice)
	api.GET("/invoices/:id", GetInvoice)
	api.POST("/invoices/:id/finalize", FinalizeInvoice)
	api.POST("/invoices/:id/pay", PayInvoice)
	api.POST("/invoices/:id/void", VoidInvoice)

	// Billing subscriptions
	api.POST("/subscriptions", CreateBillingSubscription)
	api.GET("/subscriptions", ListBillingSubscriptions)
	api.GET("/subscriptions/:id", GetBillingSubscription)
	api.PATCH("/subscriptions/:id", UpdateBillingSubscription)
	api.POST("/subscriptions/:id/cancel", CancelBillingSubscription)
	api.POST("/subscriptions/:id/reactivate", ReactivateBillingSubscription)
	api.POST("/subscriptions/:id/renew", RenewBillingSubscription)

	// Payment intents
	api.POST("/payment-intents", CreatePaymentIntent)
	api.GET("/payment-intents", ListPaymentIntents)
	api.GET("/payment-intents/:id", GetPaymentIntent)
	api.POST("/payment-intents/:id/confirm", ConfirmPaymentIntent)
	api.POST("/payment-intents/:id/capture", CapturePaymentIntent)
	api.POST("/payment-intents/:id/cancel", CancelPaymentIntent)

	// Setup intents
	api.POST("/setup-intents", CreateSetupIntent)
	api.GET("/setup-intents/:id", GetSetupIntent)
	api.POST("/setup-intents/:id/confirm", ConfirmSetupIntent)
	api.POST("/setup-intents/:id/cancel", CancelSetupIntent)

	// Payment methods
	api.POST("/payment-methods", CreatePaymentMethod)
	api.GET("/payment-methods", ListPaymentMethods)
	api.GET("/payment-methods/:id", GetPaymentMethod)
	api.PATCH("/payment-methods/:id", UpdatePaymentMethod)
	api.DELETE("/payment-methods/:id", DetachPaymentMethod)
	api.POST("/customers/:id/default-payment-method", SetDefaultPaymentMethod)

	// Subscription items
	api.POST("/subscription-items", CreateSubscriptionItem)
	api.GET("/subscription-items", ListSubscriptionItems)
	api.GET("/subscription-items/:id", GetSubscriptionItem)
	api.PATCH("/subscription-items/:id", UpdateSubscriptionItem)
	api.DELETE("/subscription-items/:id", DeleteSubscriptionItem)

	// Refunds
	api.POST("/refunds", CreateRefund)
	api.GET("/refunds", ListRefunds)
	api.GET("/refunds/:id", GetRefund)

	// Credit notes
	api.POST("/credit-notes", CreateCreditNote)
	api.GET("/credit-notes", ListCreditNotes)
	api.GET("/credit-notes/:id", GetCreditNote)
	api.POST("/credit-notes/:id/void", VoidCreditNote)

	// Disputes
	api.GET("/disputes", ListDisputes)
	api.GET("/disputes/:id", GetDispute)
	api.PATCH("/disputes/:id", SubmitDisputeEvidence)
	api.POST("/disputes/:id/close", CloseDispute)

	// Customer balance
	api.GET("/customer-balance", GetCustomerBalance)
	api.POST("/customer-balance/adjustments", AdjustCustomerBalance)
	api.GET("/balance-transactions", ListBalanceTransactions)

	// Payouts
	api.POST("/payouts", CreatePayout)
	api.GET("/payouts", ListPayouts)
	api.GET("/payouts/:id", GetPayout)
	api.POST("/payouts/:id/cancel", CancelPayout)

	// Billing events
	api.GET("/events", ListBillingEvents)
	api.GET("/events/:id", GetBillingEvent)

	// Webhook endpoints
	api.POST("/webhook-endpoints", CreateWebhookEndpoint)
	api.GET("/webhook-endpoints", ListWebhookEndpoints)
	api.GET("/webhook-endpoints/:id", GetWebhookEndpoint)
	api.PATCH("/webhook-endpoints/:id", UpdateWebhookEndpoint)
	api.DELETE("/webhook-endpoints/:id", DeleteWebhookEndpoint)

	// Customer portal
	api.GET("/portal/overview", PortalOverview)
	api.GET("/portal/invoices", PortalInvoices)
	api.GET("/portal/subscriptions", PortalSubscriptions)
	api.GET("/portal/payment-methods", PortalPaymentMethods)

	// Subscription schedules
	api.POST("/subscription-schedules", CreateSubscriptionSchedule)
	api.GET("/subscription-schedules", ListSubscriptionSchedules)
	api.GET("/subscription-schedules/:id", GetSubscriptionSchedule)
	api.PATCH("/subscription-schedules/:id", UpdateSubscriptionSchedule)
	api.POST("/subscription-schedules/:id/cancel", CancelSubscriptionSchedule)
	api.POST("/subscription-schedules/:id/release", ReleaseSubscriptionSchedule)

	// Bank transfer instructions
	api.POST("/bank-transfer-instructions", CreateBankTransferInstruction)
	api.GET("/bank-transfer-instructions", ListBankTransferInstructions)
	api.GET("/bank-transfer-instructions/:id", GetBankTransferInstruction)
	api.POST("/reconciliation/match", ReconcileInboundTransfer)

	// Invoice sub-resources
	api.POST("/invoices/:id/line-items", AddInvoiceLineItem)
	api.DELETE("/invoices/:id/line-items/:itemId", RemoveInvoiceLineItem)
	api.POST("/invoices/:id/apply-discount", ApplyInvoiceDiscount)
	api.POST("/invoices/:id/calculate-tax", CalculateInvoiceTax)

	// Capabilities
	api.GET("/capabilities", GetCapabilities)

	// ZAP protocol endpoint
	api.POST("/zap", ZapDispatch)
}
