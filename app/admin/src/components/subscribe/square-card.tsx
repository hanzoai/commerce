'use client'

import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from 'react'
import { Text } from '@hanzo/commerce-ui'
import { loadSquare, type SquareCardInstance } from './load-square'
import type { PaymentConfig } from '@/lib/commerce-client'

export interface SquareCardHandle {
  /** Tokenize the entered card → a single-use source nonce, or throw. */
  tokenize(): Promise<string>
}

/**
 * Square Web-Payments card-capture widget. Loads the CDN SDK once (idempotent
 * via loadSquare), mounts a hosted card field, and exposes an imperative
 * `tokenize()` the Subscribe button calls to mint the sourceId nonce. Kept
 * behind a dynamic import so the SDK-bound code never enters the first paint.
 */
export const SquareCard = forwardRef<SquareCardHandle, { config: PaymentConfig }>(
  function SquareCard({ config }, ref) {
    const mount = useRef<HTMLDivElement>(null)
    const card = useRef<SquareCardInstance | null>(null)
    const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading')
    const [message, setMessage] = useState('')

    useEffect(() => {
      let alive = true
      let instance: SquareCardInstance | null = null

      loadSquare(config.environment)
        .then(async (sdk) => {
          if (!alive || !mount.current) return
          const payments = sdk.payments(config.applicationId, config.locationId)
          instance = await payments.card()
          if (!alive) {
            await instance.destroy().catch(() => {})
            return
          }
          await instance.attach(mount.current)
          card.current = instance
          setStatus('ready')
        })
        .catch((cause: unknown) => {
          if (!alive) return
          setStatus('error')
          setMessage(cause instanceof Error ? cause.message : 'Could not load card payments.')
        })

      return () => {
        alive = false
        card.current = null
        instance?.destroy().catch(() => {})
      }
    }, [config.applicationId, config.locationId, config.environment])

    useImperativeHandle(ref, () => ({
      async tokenize() {
        if (!card.current) throw new Error('Card field is not ready yet.')
        const result = await card.current.tokenize()
        if (result.status !== 'OK' || !result.token) {
          throw new Error(result.errors?.[0]?.message || 'Card verification failed.')
        }
        return result.token
      },
    }))

    return (
      <div className="flex flex-col gap-y-1.5">
        <div
          ref={mount}
          className="min-h-[52px] rounded-lg border border-ui-border-base bg-ui-bg-field px-3 py-2"
        />
        {status === 'loading' && (
          <Text size="xsmall" className="text-ui-fg-muted">Loading secure card field…</Text>
        )}
        {status === 'error' && (
          <Text size="xsmall" className="text-ui-fg-error">{message}</Text>
        )}
      </div>
    )
  },
)
