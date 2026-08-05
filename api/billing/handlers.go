package billing

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
)

// Route registers billing endpoints for service-to-service calls.
// These are internal endpoints used by Cloud-API; require admin token.
// mintMountPath is the address these routes are SERVED at: api/api.go mounts this
// package's Route under /v1, and Route groups it under "billing". A router cannot
// report its own absolute address — zip composes definitions, so the same one can
// be included at more than one site — so the code that knows the mount states it.
//
// It is a claim, and TestMintRoutesMatchWhatIsServed checks it against the app's
// own declaration rather than taking its word.
const mintMountPath = "/v1/billing"

func Route(r zip.Router, args ...zip.Handler) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := r.Group("billing")
	api.Use(adminRequired)

	// mint is the money-MINT surface: every route registered through it (those
	// that credit spendable balance from a client-supplied amount) is gated on
	// the internal service token OR a platform global admin ONLY — NEVER the
	// org-level Admin bit. Without that gate, TokenRequired(permission.Admin)
	// admitted any org OWNER (org-level IAM isAdmin → Admin|Live), who could then
	// self-credit unlimited balance → unlimited free inference (the
	// real-money-GA blocker). cloud-api's service-token money path is UNAFFECTED
	// (the service-token branch grants the marker the gate checks).
	//
	// Registering here IS the declaration: middleware.Mint applies the gate and
	// records the route in middleware.MintRoutes(), so the mint surface is
	// derived from these registrations instead of hand-listed by each consumer
	// (cloud's billing-bridge allowlist must stay disjoint from it). See
	// middleware/platformonly.go and middleware/mint.go.
	mint := middleware.Mint(api, mintMountPath)

	// Tier (tier-aware billing)
	api.Get("/tier", GetTier)

	// Included monthly usage allotment (plan free-tier credit).
	// grant/run mutate; usage-rollup is the read surface for console.
	api.Post("/allotment/grant", GrantAllotment)
	api.Post("/allotment/run", RunAllotments)
	api.Get("/usage/rollup", GetUsageRollup)

	// Balance & usage (existing)
	api.Get("/balance", GetBalance)
	api.Get("/balance/all", GetBalanceAll)
	api.Get("/usage", GetUsage)
	api.Post("/usage", RecordUsage)
	// Money-MINT routes: service-token / global-admin ONLY.
	mint.Post("/deposit", Deposit)
	mint.Post("/refund", Refund)

	// Chain-backed credit ledger (HUSD): the indexer backfill/reconcile pass and
	// the metered-usage settlement sweep are platform-only (they move on-chain
	// money / write the money ledger); status is a read-only observability surface.
	mint.Post("/husd/sync", SyncHUSD)
	mint.Post("/husd/settle", SettleHUSD)
	mint.Post("/husd/migrate", MigrateHUSD)
	api.Get("/husd/status", StatusHUSD)

	// SBOM-driven OSS-developer payout.
	//   POST /sbom               — arcd build pipeline ingests an image's SBOM
	//   GET  /sbom               — list stored SBOM records
	//   GET  /oss-accruals       — per-line accrual ledger reads
	//   GET  /oss-payout/summary — per-package payout rollup (disbursement view)
	api.Post("/sbom", IngestSBOM)
	api.Get("/sbom", ListSBOMs)
	api.Get("/oss-accruals", ListOSSAccruals)
	api.Get("/oss-payout/summary", GetOSSPayoutSummary)

	// Meters
	api.Post("/meters", CreateMeter)
	api.Get("/meters", ListMeters)
	api.Get("/meters/:id", GetMeter)

	// Meter events
	api.Post("/meter-events", RecordMeterEvents)
	api.Get("/meter-events/summary", GetMeterEventsSummary)

	// Tier check (lightweight model-access gate for Chat / white-label)
	api.Get("/tier-check", TierCheck)

	// Credit — THE ONE way credit enters an org ledger (money-MINT: service-token /
	// global-admin ONLY). A client-supplied amount is exactly what must never be
	// self-service, so this is mint-gated; the on-signup "starter" credit is just a
	// parameterized call (tag=starter-credit, amountCents=500, expiry=+365d) from
	// cloud-api / chat, not a separate route. Idempotent on idempotencyKey.
	mint.Post("/credit", Credit)

	// Credit grants (money-MINT: service-token / global-admin ONLY). Reads moved
	// to the user group below. Void is a grant mutation in the same resource
	// family — same platform-only bar, so an org owner can neither create nor
	// alter a grant.
	mint.Post("/credits", CreateCreditGrant)
	mint.Post("/credits/:id/void", VoidCreditGrant)

	// Pricing rules
	api.Post("/pricing-rules", CreatePricingRule)
	api.Get("/pricing-rules", ListPricingRules)
	api.Delete("/pricing-rules/:id", DeletePricingRule)

	// Invoice preview (legacy)
	api.Post("/invoice-preview", InvoicePreview)

	// Billing invoices
	api.Post("/invoices", CreateInvoice)
	api.Get("/invoices", ListInvoices)
	api.Get("/invoices/upcoming", UpcomingInvoice)
	api.Get("/invoices/:id", GetInvoice)
	api.Post("/invoices/:id/finalize", FinalizeInvoice)
	api.Post("/invoices/:id/pay", PayInvoice)
	api.Post("/invoices/:id/void", VoidInvoice)

	// Billing subscriptions
	api.Post("/subscriptions", CreateBillingSubscription)
	api.Get("/subscriptions", ListBillingSubscriptions)
	api.Get("/subscriptions/:id", GetBillingSubscription)
	api.Patch("/subscriptions/:id", UpdateBillingSubscription)
	api.Post("/subscriptions/:id/cancel", CancelBillingSubscription)
	api.Post("/subscriptions/:id/reactivate", ReactivateBillingSubscription)
	api.Post("/subscriptions/:id/renew", RenewBillingSubscription)

	// Payment intents
	api.Post("/payment-intents", CreatePaymentIntent)
	api.Get("/payment-intents", ListPaymentIntents)
	api.Get("/payment-intents/:id", GetPaymentIntent)
	api.Post("/payment-intents/:id/confirm", ConfirmPaymentIntent)
	api.Post("/payment-intents/:id/capture", CapturePaymentIntent)
	api.Post("/payment-intents/:id/cancel", CancelPaymentIntent)

	// Setup intents
	api.Post("/setup-intents", CreateSetupIntent)
	api.Get("/setup-intents/:id", GetSetupIntent)
	api.Post("/setup-intents/:id/confirm", ConfirmSetupIntent)
	api.Post("/setup-intents/:id/cancel", CancelSetupIntent)

	// Payment methods — moved to user group below (accepts both admin & user tokens)

	// Subscription items
	api.Post("/subscription-items", CreateSubscriptionItem)
	api.Get("/subscription-items", ListSubscriptionItems)
	api.Get("/subscription-items/:id", GetSubscriptionItem)
	api.Patch("/subscription-items/:id", UpdateSubscriptionItem)
	api.Delete("/subscription-items/:id", DeleteSubscriptionItem)

	// Refunds
	api.Post("/refunds", CreateRefund)
	api.Get("/refunds", ListRefunds)
	api.Get("/refunds/:id", GetRefund)

	// Credit notes
	api.Post("/credit-notes", CreateCreditNote)
	api.Get("/credit-notes", ListCreditNotes)
	api.Get("/credit-notes/:id", GetCreditNote)
	api.Post("/credit-notes/:id/void", VoidCreditNote)

	// Disputes
	api.Get("/disputes", ListDisputes)
	api.Get("/disputes/:id", GetDispute)
	api.Patch("/disputes/:id", SubmitDisputeEvidence)
	api.Post("/disputes/:id/close", CloseDispute)

	// Customer balance (reads stay admin; the adjustment MINTS balance →
	// service-token / global-admin ONLY).
	api.Get("/customer-balance", GetCustomerBalance)
	mint.Post("/customer-balance/adjustments", AdjustCustomerBalance)
	api.Get("/balance-transactions", ListBalanceTransactions)

	// Payouts. Creating/cancelling a payout MOVES money out — money-MINT bar
	// (service-token / global-admin ONLY). Reads stay admin-scoped.
	mint.Post("/payouts", CreatePayout)
	api.Get("/payouts", ListPayouts)
	api.Get("/payouts/:id", GetPayout)
	mint.Post("/payouts/:id/cancel", CancelPayout)

	// Billing events
	api.Get("/events", ListBillingEvents)
	api.Get("/events/:id", GetBillingEvent)

	// Inbound webhook ingress (unauthenticated — signature-verified per provider).
	// Registered outside the admin-token group because providers do not carry
	// commerce admin tokens; the provider's signature is the trust anchor.
	r.Post("/billing/webhooks/:provider", HandleProviderWebhook)

	// Customer portal — the SERVICE-TOKEN face of the same customer surface the
	// `user` group below serves directly. A host that fronts commerce owns the
	// customer address itself (cloud's billing app owns /v1/billing/methods) and
	// reaches the data through here, so the portal family is what it may proxy to
	// without dispatching back into its own route.
	api.Get("/portal/overview", PortalOverview)
	api.Get("/portal/invoices", PortalInvoices)
	api.Get("/portal/subscriptions", PortalSubscriptions)
	api.Get("/portal/methods", PortalPaymentMethods)
	// Removing a saved card, at the portal address for the same reason the list is
	// here: a proxying host cannot forward DELETE /billing/methods/:id — that is the
	// address it publishes itself, so the forward re-enters its own handler. Without
	// this the sub-resource had no reachable owner and a customer could add a card
	// and never remove one (the live edge answered 405).
	//
	// SAME handler, SAME guards as the user-group route below: the org namespace
	// scopes cross-org (a foreign id is a not-found miss) and
	// callerMayReachBillingSubject closes the intra-org gap on the unpinned :id.
	api.Delete("/portal/methods/:id", DetachPaymentMethod)

	// Subscription schedules
	api.Post("/subscription-schedules", CreateSubscriptionSchedule)
	api.Get("/subscription-schedules", ListSubscriptionSchedules)
	api.Get("/subscription-schedules/:id", GetSubscriptionSchedule)
	api.Patch("/subscription-schedules/:id", UpdateSubscriptionSchedule)
	api.Post("/subscription-schedules/:id/cancel", CancelSubscriptionSchedule)
	api.Post("/subscription-schedules/:id/release", ReleaseSubscriptionSchedule)

	// Bank transfer instructions
	api.Post("/bank-transfer-instructions", CreateBankTransferInstruction)
	api.Get("/bank-transfer-instructions", ListBankTransferInstructions)
	api.Get("/bank-transfer-instructions/:id", GetBankTransferInstruction)
	// Reconciling an inbound transfer asserts "this money arrived" and credits the
	// customer's balance by a CLIENT-SUPPLIED amount, through the very same
	// engine.AdjustCustomerBalance that /customer-balance/adjustments is
	// mint-gated for. Only the platform can attest that a wire landed, so it sits
	// at the platform bar — an org owner must not be able to declare an arbitrary
	// transfer against a reference it also controls.
	//
	// Impact today is LEDGER INTEGRITY, not free inference: nothing spends a
	// customer balance. engine.ApplyBalanceToInvoice is the only function that
	// would, and it has no callers; models/customerbalance is imported by exactly
	// one file. So the reachable damage is fabricated "bank transfer received"
	// balance-transaction rows and an inflated balance — and a landmine for
	// whoever wires ApplyBalanceToInvoice up, at which point this becomes a live
	// mint. Gated now, while it is cheap.
	//
	// The gate cannot regress a caller: this route has none, in commerce or in
	// cloud. Found by the mint-surface guard, not by hand.
	mint.Post("/reconciliation/match", ReconcileInboundTransfer)

	// Invoice sub-resources
	api.Post("/invoices/:id/line-items", AddInvoiceLineItem)
	api.Delete("/invoices/:id/line-items/:itemId", RemoveInvoiceLineItem)
	api.Post("/invoices/:id/apply-discount", ApplyInvoiceDiscount)
	api.Post("/invoices/:id/calculate-tax", CalculateInvoiceTax)

	// Capabilities
	api.Get("/capabilities", GetCapabilities)

	// Top-up: charge a saved payment method and credit user balance
	api.Post("/topup", Topup)

	// GPU billing (server-enforced prepaid-only + card-required). The cloud GPU
	// launch gate reads /gpu/eligibility before provisioning and POSTs /gpu/charge
	// to debit; a GPU charge NEVER draws credit grants (see api/billing/gpu_charge.go).
	api.Get("/gpu/eligibility", GPUChargeEligibility)
	api.Post("/gpu/charge", ChargeGPU)

	// ZAP protocol endpoint
	api.Post("/zap", ZapDispatch)

	// DNS billing endpoints
	dns := r.Group("dns")
	dns.Use(adminRequired)
	dns.Post("/usage", RecordDNSUsage)
	dns.Get("/usage/summary", GetDNSUsageSummary)

	// Billing cycle automation (platform scheduler / service). Collecting a
	// cycle charges cards across orgs — money-MINT bar (service-token /
	// global-admin ONLY), never an org owner's Admin bit. run-all sweeps EVERY
	// org, so it is emphatically platform-only.
	mint.Post("/cycle/run", RunBillingCycle)
	mint.Post("/cycle/run-user", RunBillingCycleUser)
	mint.Post("/cycle/run-all", RunBillingCycleAllOrgs)

	// Auto-recharge sweep (called by the platform scheduler / CronJob): charge
	// the default card for orgs whose balance dropped below their threshold.
	// Platform-wide card charging — money-MINT bar (service-token / global-admin
	// ONLY). An org owner reaching this could sweep-charge saved cards across
	// every org.
	mint.Post("/recharge/run-all", RunAutoRechargeAllOrgs)

	// Test mode toggle: move an org between Square sandbox and production. This
	// flips whether charges hit real cards, so it is a money-mode change —
	// service-token / global-admin ONLY, never an org owner.
	mint.Post("/mode", SetOrgTestMode)

	// ── User-facing billing endpoints ─────────────────────────────────────
	// Called by billing.hanzo.ai with user OIDC tokens. Gated by a NO-MASK
	// TokenRequired(): any authenticated principal is admitted — an IAM user (via
	// the validated iam_authenticated identity) OR a non-IAM service token (via
	// the service-token branch). It deliberately does NOT require the Admin bit
	// the admin endpoints above use, so a normal user can manage their own
	// billing; per-user/per-org scoping is enforced in the handlers and by
	// EdgeAuth's billing-subject lock at the edge. (Masked gates DO enforce their
	// masks on the IAM path since v1.46.5 — see middleware/accesstoken.go.)
	userRequired := middleware.TokenRequired()

	user := r.Group("billing")
	user.Use(userRequired)

	// Invoice PDF download (user-scoped). Tenant-isolated: the handler loads the
	// invoice from the caller's OWN org namespace, so a foreign invoice id 404s.
	// A normal authenticated org member can download their own invoice; Console
	// opens this as a per-invoice download link.
	user.Get("/invoices/:id/pdf", DownloadInvoicePDF)

	// Public Square config for THIS org's Web Payments SDK (sandbox for test
	// orgs, production for live orgs) — so the browser tokenizes against the
	// same Square account commerce vaults/charges with.
	user.Get("/settings", GetPaymentConfig)

	// Wire + crypto top-up rails (user-scoped; nothing here mints — a wire
	// settles via the admin wire/credit verb on bank receipt, a crypto deposit
	// via the chain watcher on confirmations).
	user.Get("/wire", GetWireInstructions)
	user.Get("/crypto/options", GetCryptoOptions)
	user.Post("/crypto/deposit", CreateCryptoDeposit)
	user.Get("/crypto/deposit/:id", GetCryptoDeposit)

	// Plans (public catalog — cacheable, no writes).
	// CF caches for 1 hour; plans rarely change.
	user.Get("/plans", middleware.CachePublic(3600), middleware.CFCacheTags("plans"), ListPlans)
	user.Get("/plans/:id", middleware.CachePublic(3600), middleware.CFCacheTags("plans"), GetPlan)

	// DNS plans (public catalog, cacheable)
	dnsUser := r.Group("dns")
	dnsUser.Use(userRequired)
	dnsUser.Get("/plans", middleware.CachePublic(3600), middleware.CFCacheTags("dns-plans"), ListDNSPlans)

	// Auto-recharge config (user-scoped; one per org)
	user.Get("/recharge", GetAutoRecharge)
	user.Put("/recharge", SetAutoRecharge)

	// Spend alerts + per-scope spend caps (issue #70). CRUD manages the budget
	// rows; /authorize is the per-request cap verdict the cloud metering gate
	// consumes (service-token S2S). Org-scoped via X-Org-Id, so a cap on org X
	// can never gate org Y.
	user.Get("/alerts", ListSpendAlerts)
	user.Post("/alerts", CreateSpendAlert)
	user.Get("/alerts/authorize", AuthorizeSpendCap)
	user.Patch("/alerts/:id", UpdateSpendAlert)
	user.Delete("/alerts/:id", DeleteSpendAlert)

	// Billing status — hasPaymentMethod + creditBalance in one call (used by bot gateway)
	user.Get("/status", GetBillingStatus)

	// Self-service balance read. Identity comes from the gateway-injected
	// X-Org-Id / X-User-Id headers; no admin token required. Granting credit is
	// NOT self-service — it is the mint-gated POST /billing/credit above, so a
	// user can read their balance but never mint into it.
	user.Get("/me/balance", GetMyBalance)

	// Credit grants & balance (read-only, user-scoped)
	user.Get("/credits", ListCreditGrants)
	user.Get("/credit-balance", GetCreditBalance)
	user.Get("/credit-balance/breakdown", GetCreditBalanceBreakdown)

	// Transaction history / ledger (read-only, user-scoped). Derives identity
	// from the IAM org/user in context like the sibling reads above. Called by
	// billing.hanzo.ai's Transactions tab as GET /v1/billing/transactions.
	// Registered here so it lives under the CORS-enabled API group; an
	// unregistered route hits gin NoRoute (404, no Access-Control-Allow-Origin)
	// and the browser reports it as a CORS failure rather than an honest empty list.
	user.Get("/transactions", ListTransactions)

	// Withdraw (user-initiated: move funds out of Commerce balance).
	// Used by bot wallet funding (source=usd) to deduct from user's account.
	// Non-admin callers may only withdraw from their own account.
	user.Post("/withdraw", Withdraw)

	// Top-up with a Square Web Payments SDK nonce (no saved PM required)
	user.Post("/topup/token", TopupWithToken)

	// TRUE card-on-file monthly subscription: vault a Square nonce as a reusable
	// card, charge the first period at the SERVER-AUTHORITATIVE plan price, and
	// create the subscription — all server-side, authorized by the settled charge
	// (not the paid-tier mint gate). Same user group as topup/token: any
	// authenticated org member may subscribe by paying with a card.
	user.Post("/subscribe/card", SubscribeWithCard)

	// Payment methods (user-scoped CRUD)
	user.Post("/methods", CreatePaymentMethod)
	user.Get("/methods", ListPaymentMethods)
	user.Get("/methods/:id", GetPaymentMethod)
	user.Patch("/methods/:id", UpdatePaymentMethod)
	user.Delete("/methods/:id", DetachPaymentMethod)
	user.Post("/customers/:id/default-payment-method", SetDefaultPaymentMethod)

	// Billing accounts (org-wrapper)
	user.Get("/accounts", ListBillingAccounts)
	user.Post("/accounts", CreateBillingAccount)
	user.Get("/accounts/:id/members", ListAccountMembers)
	user.Post("/accounts/:id/members", AddAccountMember)
	user.Patch("/accounts/:id/members/:memberId", UpdateMemberRole)
	user.Delete("/accounts/:id/members/:memberId", RemoveAccountMember)
}
