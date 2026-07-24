'use client'

import dynamic from 'next/dynamic'

// The paywall carries react-hook-form + zod + the Square SDK loader — keep it
// out of the shared bundle and stream it in on this route only.
const Paywall = dynamic(() => import('@/components/subscribe/paywall').then((m) => m.Paywall), {
  ssr: false,
  loading: () => (
    <div className="w-full max-w-2xl">
      <div className="mx-auto mb-8 h-24 w-64 animate-pulse rounded-lg bg-ui-bg-component" />
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {[0, 1].map((i) => (
          <div key={i} className="h-40 animate-pulse rounded-xl bg-ui-bg-component" />
        ))}
      </div>
      <div className="mt-6 h-64 animate-pulse rounded-xl bg-ui-bg-component" />
    </div>
  ),
})

export default function SubscribePage() {
  return <Paywall />
}
