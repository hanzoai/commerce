// ── Commerce API client ─────────────────────────────────────────────────────
export { Commerce, CommerceApiError } from './client'

// ── IAM auth utilities ──────────────────────────────────────────────────────
export {
  isLoggedIn,
  getAccessToken,
  getCurrentUser,
  startLogin,
  handleCallback,
  logout,
} from './auth'
export type { IamUser, IamAuthConfig } from './auth'

// ── Configuration ───────────────────────────────────────────────────────────
export {
  getOrgByHost,
  isAdminUser,
  mergePlans,
  HANZO_PLANS,
  DEFAULT_COMMERCE_URL,
  DEFAULT_IAM_SERVER_URL,
  DEFAULT_IAM_CLIENT_ID,
} from './config'
export type { CommerceOrgConfig } from './config'

// ── Types ───────────────────────────────────────────────────────────────────
export type {
  // Billing / subscription plan types
  BillingInterval,
  SubscriptionPlan,
  Subscription,
  SubscriptionDiscount,
  RetentionOffer,
  SubscriptionHistory,
  BillingMetric,

  // Business profile types
  BusinessAddress,
  BusinessContact,
  BusinessProfile,

  // Tax and compliance types
  TaxRegistration,
  TaxSettings,
  ComplianceItem,

  // Payment types
  PaymentMethodType,
  CryptoNetwork,
  PaymentMethod,

  // Invoice types
  Invoice,
  InvoiceItem,
  InvoiceFilters,

  // Spend alert types
  SpendAlert,

  // Usage tracking types
  UsageMeterType,
  UsageRecord,
  UsageSummary,

  // Credit grant types
  CreditGrant,

  // Transaction record types
  TransactionType,
  TransactionRecord,

  // Support tier types
  SupportTier,

  // Discount / promotion code
  DiscountCode,

  // Commerce API wire types
  CommerceClientConfig,
  Balance,
  CommerceTransaction,
  CommerceSubscription,
  CommercePlan,
  CommercePayment,
  CommerceInvoice,
  CommercePaymentMethod,
  CommerceSpendAlert,
  CommerceUsageRecord,
  CommerceUsageSummary,
  CommerceDiscountCode,
  CommerceCreditGrant,
  CommerceCreditBalance,
  CommerceMeter,
  CommerceMeterEventsSummary,
  CommercePortalOverview,
} from './types'
