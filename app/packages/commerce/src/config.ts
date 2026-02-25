import type { SubscriptionPlan } from './types'

// ── Organization config ─────────────────────────────────────────────────────

export interface CommerceOrgConfig {
  id: string
  displayName: string
  iamServerUrl: string
  iamClientId: string
  commerceUrl: string
  logo: string
  theme: {
    brand: string
    brandHover: string
  }
}

const hanzo: CommerceOrgConfig = {
  id: 'hanzo',
  displayName: 'Hanzo',
  iamServerUrl: 'https://hanzo.id',
  iamClientId: 'hanzo-app-client-id',
  commerceUrl: 'https://api.commerce.hanzo.ai',
  logo: '/logos/hanzo.svg',
  theme: {
    brand: '#fd4444',
    brandHover: '#e03e3e',
  },
}

/** Emails with admin/super-user billing access (can grant credits to anyone). */
const ADMIN_EMAILS = [
  'admin@hanzo.ai',
  'zach@hanzo.ai',
  'ant@hanzo.ai',
]

const organizations: Record<string, CommerceOrgConfig> = { hanzo }

/** Resolve an org config by hostname (falls back to Hanzo). */
export function getOrgByHost(host: string): CommerceOrgConfig {
  for (const [, org] of Object.entries(organizations)) {
    if (host.includes(org.id)) return org
  }
  return hanzo
}

/** Check whether an email has billing admin privileges. */
export function isAdminUser(email: string): boolean {
  return ADMIN_EMAILS.includes(email.toLowerCase())
}

/** Default Commerce API base URL. */
export const DEFAULT_COMMERCE_URL = 'https://api.commerce.hanzo.ai'

/** Default IAM server URL. */
export const DEFAULT_IAM_SERVER_URL = 'https://hanzo.id'

/** Default IAM client ID. */
export const DEFAULT_IAM_CLIENT_ID = 'hanzo-app-client-id'

// ── Canonical subscription plans ────────────────────────────────────────────

export const HANZO_PLANS: SubscriptionPlan[] = [
  {
    id: 'cloud:hobby',
    name: 'Hobby',
    description: 'For personal projects and experimentation',
    price: 0,
    billingPeriod: 'monthly',
    currency: 'usd',
    features: [
      '1,000 API requests/month',
      '7-day data retention',
      '1 project',
      'Community support',
    ],
    limits: {
      apiRequests: 1_000,
      dataRetentionDays: 7,
      projects: 1,
    },
  },
  {
    id: 'cloud:core',
    name: 'Core',
    description: 'For growing teams shipping to production',
    price: 29,
    billingPeriod: 'monthly',
    currency: 'usd',
    features: [
      '100k API requests/month included',
      '90-day data retention',
      'Unlimited users',
      'Email and chat support',
      'Overage: $8 per 100k requests',
    ],
    limits: {
      apiRequests: 100_000,
      dataRetentionDays: 90,
      projects: 5,
    },
  },
  {
    id: 'cloud:pro',
    name: 'Pro',
    description: 'For scaling workloads with compliance needs',
    price: 199,
    billingPeriod: 'monthly',
    currency: 'usd',
    highlighted: true,
    badge: 'Popular',
    features: [
      'Everything in Core',
      '3-year data retention',
      'Unlimited queues',
      'Data retention management',
      'High rate limits',
      'SOC2 and ISO 27001 reports',
      'Overage: $6 per 100k requests',
    ],
    limits: {
      apiRequests: 500_000,
      dataRetentionDays: 1095,
      projects: 20,
    },
  },
  {
    id: 'cloud:team',
    name: 'Team',
    description: 'For organizations with SSO and RBAC requirements',
    price: 499,
    billingPeriod: 'monthly',
    currency: 'usd',
    features: [
      'Everything in Pro',
      'Enterprise SSO (Okta, SAML)',
      'SSO enforcement',
      'Fine-grained RBAC',
      'Priority Slack support',
      'Dedicated success manager',
    ],
    limits: {
      apiRequests: 2_000_000,
      dataRetentionDays: 1095,
      projects: 50,
    },
  },
  {
    id: 'cloud:enterprise',
    name: 'Enterprise',
    description: 'For mission-critical deployments with custom SLAs',
    price: 2499,
    billingPeriod: 'monthly',
    currency: 'usd',
    features: [
      'Everything in Team',
      'Audit logs',
      'SCIM API for user provisioning',
      'Custom rate limits',
      '99.99% uptime SLA',
      'Dedicated support engineer',
      'Custom contract and invoicing',
    ],
    limits: {
      apiRequests: -1,
      dataRetentionDays: 1825,
      projects: -1,
    },
  },
]

/**
 * Merge server-side plans from Commerce API with local feature definitions.
 * Commerce API returns pricing/interval but not features; local config has features.
 */
export function mergePlans(
  commercePlans: SubscriptionPlan[],
  localPlans: SubscriptionPlan[] = HANZO_PLANS,
): SubscriptionPlan[] {
  if (commercePlans.length === 0) return localPlans

  return localPlans.map((local) => {
    const remote = commercePlans.find((p) => p.id === local.id || p.name === local.name)
    if (remote) {
      return {
        ...local,
        price: remote.price || local.price,
        billingPeriod: remote.billingPeriod || local.billingPeriod,
        currency: remote.currency || local.currency,
        priceId: remote.priceId || local.priceId,
      }
    }
    return local
  })
}
