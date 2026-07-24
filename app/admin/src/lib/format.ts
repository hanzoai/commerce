// Display + parse helpers shared across resource pages. Money is stored as
// integer cents; currency codes are lowercase ISO-4217 for fiat (Intl uppercases
// them). Non-ISO codes (crypto) fall back to `amount CODE` instead of throwing.

export function formatMoney(cents?: number | null, currency?: string | null): string {
  if (cents == null) return '—'
  const code = (currency || 'usd').toUpperCase()
  const amount = cents / 100
  try {
    return new Intl.NumberFormat('en-US', { style: 'currency', currency: code }).format(amount)
  } catch {
    return `${amount.toFixed(2)} ${code}`
  }
}

/** Parse a human amount string ("12.50") into integer cents. */
export function amountToCents(input: string | number): number {
  const n = typeof input === 'number' ? input : parseFloat(input)
  if (!Number.isFinite(n)) return 0
  return Math.round(n * 100)
}

/** Render integer cents as an editable amount string ("12.50"). */
export function centsToAmount(cents?: number | null): string {
  if (cents == null) return ''
  return (cents / 100).toFixed(2)
}

export function formatDate(value?: string | number | Date | null): string {
  if (!value) return '—'
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? '—' : d.toLocaleString()
}

/** "on-hold" -> "On Hold", "paymentStatus" left intact enough for labels. */
export function titleCase(value?: string | null): string {
  if (!value) return ''
  return value.replace(/[-_]/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}
