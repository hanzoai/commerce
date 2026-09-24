package billing

import (
	"net/http"

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

func Route(r *zip.Group, args ...zip.Handler) {
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
	api.Raw(http.MethodGet, "/tier", GetTier)

	// Included monthly usage allotment (plan free-tier credit).
	// grant/run mutate; usage-rollup is the read surface for console.
	api.Raw(http.MethodPost, "/allotment/grant", GrantAllotment)
	api.Raw(http.MethodPost, "/allotment/run", RunAllotments)
	api.Raw(http.MethodGet, "/usage/rollup", GetUsageRollup)

	// Balance & usage (existing)
	api.Raw(http.MethodGet, "/balance", GetBalance)
	api.Raw(http.MethodGet, "/balance/all", GetBalanceAll)
	api.Raw(http.MethodGet, "/usage", GetUsage)
	api.Raw(http.MethodPost, "/usage", RecordUsage)
	// Money-MINT routes: service-token / global-admin ONLY.
	mint.Raw(http.MethodPost, "/deposit", Deposit)
	mint.Raw(http.MethodPost, "/refund", Refund)

	// Chain-backed credit ledger (HUSD): the indexer backfill/reconcile pass and
	// the metered-usage settlement sweep are platform-only (they move on-chain
	// money / write the money ledger); status is a read-only observability surface.
	mint.Raw(http.MethodPost, "/husd/sync", SyncHUSD)
	mint.Raw(http.MethodPost, "/husd/settle", SettleHUSD)
	mint.Raw(http.MethodPost, "/husd/migrate", MigrateHUSD)
	api.Raw(http.MethodGet, "/husd/status", StatusHUSD)

	// SBOM-driven OSS-developer payout.
	//   POST /sbom               — arcd build pipeline ingests an image's SBOM
	//   GET  /sbom               — list stored SBOM records
	//   GET  /oss-accruals       — per-line accrual ledger reads
	//   GET  /oss-payout/summary — per-package payout rollup (disbursement view)
	api.Raw(http.MethodPost, "/sbom", IngestSBOM)
	api.Raw(http.MethodGet, "/sbom", ListSBOMs)
	api.Raw(http.MethodGet, "/oss-accruals", ListOSSAccruals)
	api.Raw(http.MethodGet, "/oss-payout/summary", GetOSSPayoutSummary)

	// Meters
	api.Raw(http.MethodPost, "/meters", CreateMeter)
	api.Raw(http.MethodGet, "/meters", ListMeters)
	api.Raw(http.MethodGet, "/meters/:id", GetMeter)

	// Meter events
	api.Raw(http.MethodPost, "/meter-events", RecordMeterEvents)
	api.Raw(http.MethodGet, "/meter-events/summary", GetMeterEventsSummary)

	// Tier check (lightweight model-access gate for Chat / white-label)
	api.Raw(http.MethodGet, "/tier-check", TierCheck)

	// Credit — THE ONE way credit enters an org ledger (money-MINT: service-token /
	// global-admin ONLY). A client-supplied amount is exactly what must never be
	// self-service, so this is mint-gated; the on-signup "starter" credit is just a
	// parameterized call (tag=starter-credit, amountCents=500, expiry=+365d) from
	// cloud-api / chat, not a separate route. Idempotent on idempotencyKey.
	mint.Raw(http.MethodPost, "/credit", Credit)

	// Credit grants (money-MINT: service-token / global-admin ONLY). Reads moved
	// to the user group below. Void is a grant mutation in the same resource
	// family — same platform-only bar, so an org owner can neither create nor
	// alter a grant.
	mint.Raw(http.MethodPost, "/credits", CreateCreditGrant)
	mint.Raw(http.MethodPost, "/credits/:id/void", VoidCreditGrant)

	// Pricing rules
	api.Raw(http.MethodPost, "/pricing-rules", CreatePricingRule)
	api.Raw(http.MethodGet, "/pricing-rules", ListPricingRules)
	api.Raw(http.MethodDelete, "/pricing-rules/:id", DeletePricingRule)

	// Invoice preview (legacy)
	api.Raw(http.MethodPost, "/invoice-preview", InvoicePreview)

	// Billing invoices
	api.Raw(http.MethodPost, "/invoices", CreateInvoice)
	api.Raw(http.MethodGet, "/invoices", ListBillingInvoices)
	api.Raw(http.MethodGet, "/invoices/upcoming", UpcomingInvoice)
	api.Raw(http.MethodGet, "/invoices/:id", GetInvoice)
	api.Raw(http.MethodPost, "/invoices/:id/finalize", FinalizeInvoice)
	api.Raw(http.MethodPost, "/invoices/:id/pay", PayInvoice)
	api.Raw(http.MethodPost, "/invoices/:id/void", VoidInvoice)

	// Billing subscriptions
	api.Raw(http.MethodPost, "/subscriptions", CreateBillingSubscription)
	api.Raw(http.MethodGet, "/subscriptions", ListBillingSubscriptions)
	api.Raw(http.MethodGet, "/subscriptions/:id", GetBillingSubscription)
	api.Raw(http.MethodPatch, "/subscriptions/:id", UpdateBillingSubscription)
	api.Raw(http.MethodPost, "/subscriptions/:id/cancel", CancelBillingSubscription)
	api.Raw(http.MethodPost, "/subscriptions/:id/reactivate", ReactivateBillingSubscription)
	api.Raw(http.MethodPost, "/subscriptions/:id/renew", RenewBillingSubscription)

	// Payment intents
	api.Raw(http.MethodPost, "/payment-intents", CreatePaymentIntent)
	api.Raw(http.MethodGet, "/payment-intents", ListPaymentIntents)
	api.Raw(http.MethodGet, "/payment-intents/:id", GetPaymentIntent)
	api.Raw(http.MethodPost, "/payment-intents/:id/confirm", ConfirmPaymentIntent)
	api.Raw(http.MethodPost, "/payment-intents/:id/capture", CapturePaymentIntent)
	api.Raw(http.MethodPost, "/payment-intents/:id/cancel", CancelPaymentIntent)

	// Setup intents
	api.Raw(http.MethodPost, "/setup-intents", CreateSetupIntent)
	api.Raw(http.MethodGet, "/setup-intents/:id", GetSetupIntent)
	api.Raw(http.MethodPost, "/setup-intents/:id/confirm", ConfirmSetupIntent)
	api.Raw(http.MethodPost, "/setup-intents/:id/cancel", CancelSetupIntent)

	// Payment methods — moved to user group below (accepts both admin & user tokens)

	// Subscription items
	api.Raw(http.MethodPost, "/subscription-items", CreateSubscriptionItem)
	api.Raw(http.MethodGet, "/subscription-items", ListSubscriptionItems)
	api.Raw(http.MethodGet, "/subscription-items/:id", GetSubscriptionItem)
	api.Raw(http.MethodPatch, "/subscription-items/:id", UpdateSubscriptionItem)
	api.Raw(http.MethodDelete, "/subscription-items/:id", DeleteSubscriptionItem)

	// Refunds
	api.Raw(http.MethodPost, "/refunds", CreateRefund)
	api.Raw(http.MethodGet, "/refunds", ListRefunds)
	api.Raw(http.MethodGet, "/refunds/:id", GetRefund)

	// Credit notes
	api.Raw(http.MethodPost, "/credit-notes", CreateCreditNote)
	api.Raw(http.MethodGet, "/credit-notes", ListCreditNotes)
	api.Raw(http.MethodGet, "/credit-notes/:id", GetCreditNote)
	api.Raw(http.MethodPost, "/credit-notes/:id/void", VoidCreditNote)

	// Disputes
	api.Raw(http.MethodGet, "/disputes", ListDisputes)
	api.Raw(http.MethodGet, "/disputes/:id", GetDispute)
	api.Raw(http.MethodPatch, "/disputes/:id", SubmitDisputeEvidence)
	api.Raw(http.MethodPost, "/disputes/:id/close", CloseDispute)

	// Customer balance (reads stay admin; the adjustment MINTS balance →
	// service-token / global-admin ONLY).
	api.Raw(http.MethodGet, "/customer-balance", GetCustomerBalance)
	mint.Raw(http.MethodPost, "/customer-balance/adjustments", AdjustCustomerBalance)
	api.Raw(http.MethodGet, "/balance-transactions", ListBalanceTransactions)

	// Payouts. Creating/cancelling a payout MOVES money out — money-MINT bar
	// (service-token / global-admin ONLY). Reads stay admin-scoped.
	mint.Raw(http.MethodPost, "/payouts", CreatePayout)
	api.Raw(http.MethodGet, "/payouts", ListBillingPayouts)
	api.Raw(http.MethodGet, "/payouts/:id", GetPayout)
	mint.Raw(http.MethodPost, "/payouts/:id/cancel", CancelPayout)

	// Billing events
	api.Raw(http.MethodGet, "/events", ListBillingEvents)
	api.Raw(http.MethodGet, "/events/:id", GetBillingEvent)

	// Inbound webhook ingress (unauthenticated — signature-verified per provider).
	// Registered outside the admin-token group because providers do not carry
	// commerce admin tokens; the provider's signature is the trust anchor.
	r.Raw(http.MethodPost, "/billing/webhooks/:provider", HandleProviderWebhook)

	// Customer portal — the SERVICE-TOKEN face of the same customer surface the
	// `user` group below serves directly. A host that fronts commerce owns the
	// customer address itself (cloud's billing app owns /v1/billing/methods) and
	// reaches the data through here, so the portal family is what it may proxy to
	// without dispatching back into its own route.
	api.Raw(http.MethodGet, "/portal/overview", PortalOverview)
	api.Raw(http.MethodGet, "/portal/invoices", PortalInvoices)
	api.Raw(http.MethodGet, "/portal/subscriptions", PortalSubscriptions)
	api.Raw(http.MethodGet, "/portal/methods", PortalPaymentMethods)
	// Removing a saved card, at the portal address for the same reason the list is
	// here: a proxying host cannot forward DELETE /billing/methods/:id — that is the
	// address it publishes itself, so the forward re-enters its own handler. Without
	// this the sub-resource had no reachable owner and a customer could add a card
	// and never remove one (the live edge answered 405).
	//
	// SAME handler, SAME guards as the user-group route below: the org namespace
	// scopes cross-org (a foreign id is a not-found miss) and
	// callerMayReachBillingSubject closes the intra-org gap on the unpinned :id.
	api.Raw(http.MethodDelete, "/portal/methods/:id", DetachPaymentMethod)

	// Subscription schedules
	api.Raw(http.MethodPost, "/subscription-schedules", CreateSubscriptionSchedule)
	api.Raw(http.MethodGet, "/subscription-schedules", ListSubscriptionSchedules)
	api.Raw(http.MethodGet, "/subscription-schedules/:id", GetSubscriptionSchedule)
	api.Raw(http.MethodPatch, "/subscription-schedules/:id", UpdateSubscriptionSchedule)
	api.Raw(http.MethodPost, "/subscription-schedules/:id/cancel", CancelSubscriptionSchedule)
	api.Raw(http.MethodPost, "/subscription-schedules/:id/release", ReleaseSubscriptionSchedule)

	// Bank transfer instructions
	api.Raw(http.MethodPost, "/bank-transfer-instructions", CreateBankTransferInstruction)
	api.Raw(http.MethodGet, "/bank-transfer-instructions", ListBankTransferInstructions)
	api.Raw(http.MethodGet, "/bank-transfer-instructions/:id", GetBankTransferInstruction)
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
	mint.Raw(http.MethodPost, "/reconciliation/match", ReconcileInboundTransfer)

	// Invoice sub-resources
	api.Raw(http.MethodPost, "/invoices/:id/line-items", AddInvoiceLineItem)
	api.Raw(http.MethodDelete, "/invoices/:id/line-items/:itemId", RemoveInvoiceLineItem)
	api.Raw(http.MethodPost, "/invoices/:id/apply-discount", ApplyInvoiceDiscount)
	api.Raw(http.MethodPost, "/invoices/:id/calculate-tax", CalculateInvoiceTax)

	// Capabilities
	api.Raw(http.MethodGet, "/capabilities", GetCapabilities)

	// Top-up: charge a saved payment method and credit user balance
	api.Raw(http.MethodPost, "/topup", Topup)

	// ZAP protocol endpoint
	api.Raw(http.MethodPost, "/zap", ZapDispatch)

	// DNS billing endpoints
	dns := r.Group("dns")
	dns.Use(adminRequired)
	dns.Raw(http.MethodPost, "/usage", RecordDNSUsage)
	dns.Raw(http.MethodGet, "/usage/summary", GetDNSUsageSummary)

	// Billing cycle automation (platform scheduler / service). Collecting a
	// cycle charges cards across orgs — money-MINT bar (service-token /
	// global-admin ONLY), never an org owner's Admin bit. run-all sweeps EVERY
	// org, so it is emphatically platform-only.
	// Each is a dry run that reports and writes nothing unless the request says
	// dryRun=false.
	mint.Raw(http.MethodPost, "/cycle/run", RunBillingCycle)
	mint.Raw(http.MethodPost, "/cycle/run-user", RunBillingCycleUser)
	mint.Raw(http.MethodPost, "/cycle/run-all", RunBillingCycleAllOrgs)

	// Correcting the dates of subscriptions a period ahead of their paid invoice
	// rewrites what the cycle bills next — platform-only, like the cycle, and a
	// dry run unless the request says dryRun=false.
	mint.Raw(http.MethodPost, "/realign/run", RealignSubscriptions)
	mint.Raw(http.MethodPost, "/realign/run-all", RealignSubscriptionsAllOrgs)

	// An invoice whose payment attempt has no known outcome is voided only by a
	// platform operator who has settled the attempt with the processor, and the
	// act is recorded in the billing event ledger.
	mint.Raw(http.MethodPost, "/invoices/:id/void-unresolved", VoidUnresolvedInvoice)

	// Auto-recharge sweep (called by the platform scheduler / CronJob): charge
	// the default card for orgs whose balance dropped below their threshold.
	// Platform-wide card charging — money-MINT bar (service-token / global-admin
	// ONLY). An org owner reaching this could sweep-charge saved cards across
	// every org.
	mint.Raw(http.MethodPost, "/recharge/run-all", RunAutoRechargeAllOrgs)

	// Test mode toggle: move an org between Square sandbox and production. This
	// flips whether charges hit real cards, so it is a money-mode change —
	// service-token / global-admin ONLY, never an org owner.
	mint.Raw(http.MethodPost, "/mode", SetOrgTestMode)

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
	user.Raw(http.MethodGet, "/invoices/:id/pdf", DownloadInvoicePDF)

	// Public Square config for THIS org's Web Payments SDK (sandbox for test
	// orgs, production for live orgs) — so the browser tokenizes against the
	// same Square account commerce vaults/charges with.
	user.Raw(http.MethodGet, "/settings", GetPaymentConfig)

	// Wire + crypto top-up rails (user-scoped; nothing here mints — a wire
	// settles via the admin wire/credit verb on bank receipt, a crypto deposit
	// via the chain watcher on confirmations).
	user.Raw(http.MethodGet, "/wire", GetBillingWireInstructions)
	user.Raw(http.MethodGet, "/crypto/options", GetBillingCryptoOptions)
	user.Raw(http.MethodPost, "/crypto/deposit", CreateBillingCryptoDeposit)
	user.Raw(http.MethodGet, "/crypto/deposit/:id", GetBillingCryptoDeposit)

	// Plans (public catalog — cacheable, no writes).
	// CF caches for 1 hour; plans rarely change.
	user.Raw(http.MethodGet, "/plans", middleware.CachePublic(3600), middleware.CFCacheTags("plans"), ListPlans)
	user.Raw(http.MethodGet, "/plans/:id", middleware.CachePublic(3600), middleware.CFCacheTags("plans"), GetPlan)

	// DNS plans (public catalog, cacheable)
	dnsUser := r.Group("dns")
	dnsUser.Use(userRequired)
	dnsUser.Raw(http.MethodGet, "/plans", middleware.CachePublic(3600), middleware.CFCacheTags("dns-plans"), ListDNSPlans)

	// Auto-recharge config (user-scoped; one per org)
	user.Raw(http.MethodGet, "/recharge", GetAutoRecharge)
	user.Raw(http.MethodPut, "/recharge", SetAutoRecharge)

	// Spend alerts + per-scope spend caps (issue #70). CRUD manages the budget
	// rows; /authorize is the per-request cap verdict the cloud metering gate
	// consumes (service-token S2S). Org-scoped via X-Org-Id, so a cap on org X
	// can never gate org Y.
	user.Raw(http.MethodGet, "/alerts", ListSpendAlerts)
	user.Raw(http.MethodPost, "/alerts", CreateSpendAlert)
	user.Raw(http.MethodGet, "/alerts/authorize", AuthorizeSpendCap)
	user.Raw(http.MethodPatch, "/alerts/:id", UpdateSpendAlert)
	user.Raw(http.MethodDelete, "/alerts/:id", DeleteSpendAlert)

	// Billing status — hasPaymentMethod + creditBalance in one call (used by bot gateway)
	user.Raw(http.MethodGet, "/status", GetBillingStatus)

	// Self-service balance read. Identity comes from the gateway-injected
	// X-Org-Id / X-User-Id headers; no admin token required. Granting credit is
	// NOT self-service — it is the mint-gated POST /billing/credit above, so a
	// user can read their balance but never mint into it.
	user.Raw(http.MethodGet, "/me/balance", GetMyBalance)

	// Credit grants & balance (read-only, user-scoped)
	user.Raw(http.MethodGet, "/credits", ListBillingCreditGrants)
	user.Raw(http.MethodGet, "/credit-balance", GetCreditBalance)
	user.Raw(http.MethodGet, "/credit-balance/breakdown", GetCreditBalanceBreakdown)

	// Transaction history / ledger (read-only, user-scoped). Derives identity
	// from the IAM org/user in context like the sibling reads above. Called by
	// billing.hanzo.ai's Transactions tab as GET /v1/billing/transactions.
	// Registered here so it lives under the CORS-enabled API group; an
	// unregistered route hits gin NoRoute (404, no Access-Control-Allow-Origin)
	// and the browser reports it as a CORS failure rather than an honest empty list.
	user.Raw(http.MethodGet, "/transactions", ListBillingTransactions)

	// Withdraw (user-initiated: move funds out of Commerce balance).
	// Used by bot wallet funding (source=usd) to deduct from user's account.
	// Non-admin callers may only withdraw from their own account.
	user.Raw(http.MethodPost, "/withdraw", Withdraw)

	// Top-up with a Square Web Payments SDK nonce (no saved PM required)
	user.Raw(http.MethodPost, "/topup/token", TopupWithToken)

	// TRUE card-on-file monthly subscription: vault a Square nonce as a reusable
	// card, charge the first period at the SERVER-AUTHORITATIVE plan price, and
	// create the subscription — all server-side, authorized by the settled charge
	// (not the paid-tier mint gate). Same user group as topup/token: any
	// authenticated org member may subscribe by paying with a card.
	user.Raw(http.MethodPost, "/subscribe/card", SubscribeWithCard)

	// Payment methods (user-scoped CRUD)
	user.Raw(http.MethodPost, "/methods", CreatePaymentMethod)
	user.Raw(http.MethodGet, "/methods", ListPaymentMethods)
	user.Raw(http.MethodGet, "/methods/:id", GetPaymentMethod)
	user.Raw(http.MethodPatch, "/methods/:id", UpdatePaymentMethod)
	user.Raw(http.MethodDelete, "/methods/:id", DetachPaymentMethod)
	user.Raw(http.MethodPost, "/customers/:id/default-payment-method", SetDefaultPaymentMethod)

	// Billing accounts (org-wrapper)
	user.Raw(http.MethodGet, "/accounts", ListBillingAccounts)
	user.Raw(http.MethodPost, "/accounts", CreateBillingAccount)
	user.Raw(http.MethodGet, "/accounts/:id/members", ListAccountMembers)
	user.Raw(http.MethodPost, "/accounts/:id/members", AddAccountMember)
	user.Raw(http.MethodPatch, "/accounts/:id/members/:memberId", UpdateMemberRole)
	user.Raw(http.MethodDelete, "/accounts/:id/members/:memberId", RemoveAccountMember)
}
