// Per-org "onboarding dismissed" flag — the ONE signal that decides whether a
// storeless org is auto-routed to the setup wizard. A brand-new org lands on
// /onboarding automatically; once the merchant finishes (launch) or opts out
// ("Skip for now"), the org is marked dismissed so the dashboard stops bouncing
// them. Keyed per org so a second brand-new org still gets its own first run.
//
// localStorage only (no cookies / URL params) — matches the wizard's own
// progress store and keeps `output: 'export'` free of Suspense boundaries.
const KEY = 'hanzo.commerce.onboarding.dismissed'

function read(): Record<string, boolean> {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as unknown
    return parsed && typeof parsed === 'object' ? (parsed as Record<string, boolean>) : {}
  } catch {
    return {}
  }
}

export function isOnboardingDismissed(org: string | null | undefined): boolean {
  if (!org) return false
  return read()[org] === true
}

export function dismissOnboarding(org: string | null | undefined): void {
  if (!org) return
  try {
    const map = read()
    map[org] = true
    localStorage.setItem(KEY, JSON.stringify(map))
  } catch {
    // Private mode / disabled storage — the wizard still works, it just won't
    // remember the dismissal across reloads.
  }
}
