'use client'

// The ONE Square Web Payments card-entry hook. Every surface that needs a
// tokenized card nonce — add a saved card, top up with a new card, subscribe —
// composes this instead of re-loading the Square SDK and re-attaching a card
// field. Given a Commerce client it resolves the org's public Square config
// (sandbox vs production), lazy-loads the SDK, attaches a card field into a
// caller-owned container, and hands back a `tokenize()` that yields a single-use
// nonce. No copy-paste of the SDK bootstrap anywhere.

import { useCallback, useRef, useState } from 'react'
import type { Commerce } from '@/lib/commerce-client'

interface SquareCard {
  attach(selector: string): Promise<void>
  tokenize(): Promise<{ status: string; token?: string; errors?: { message?: string }[] }>
  destroy?(): void
}

interface SquarePayments {
  card(): Promise<SquareCard>
}

interface SquareSdk {
  payments(applicationId: string, locationId: string): SquarePayments
}

function squareSdk(): SquareSdk | undefined {
  return (window as unknown as { Square?: SquareSdk }).Square
}

export function useSquareCard(client: Commerce) {
  const cardRef = useRef<SquareCard | null>(null)
  const [ready, setReady] = useState(false)
  const [mounting, setMounting] = useState(false)
  const [error, setError] = useState('')

  const mount = useCallback(
    async (containerId: string) => {
      if (cardRef.current) return
      setMounting(true)
      setError('')
      try {
        const config = await client.getPaymentConfig()
        if (!config?.applicationId || !config.locationId) throw new Error('Card payments are not configured for this store.')
        if (!squareSdk()) {
          await new Promise<void>((resolve, reject) => {
            const script = document.createElement('script')
            script.src =
              config.environment === 'sandbox'
                ? 'https://sandbox.web.squarecdn.com/v1/square.js'
                : 'https://web.squarecdn.com/v1/square.js'
            script.onload = () => resolve()
            script.onerror = () => reject(new Error('Could not load card payments.'))
            document.head.appendChild(script)
          })
        }
        const sdk = squareSdk()
        if (!sdk) throw new Error('Could not load card payments.')
        const payments = sdk.payments(config.applicationId, config.locationId)
        const card = await payments.card()
        await card.attach(`#${containerId}`)
        cardRef.current = card
        setReady(true)
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : 'Could not start card entry.')
      } finally {
        setMounting(false)
      }
    },
    [client],
  )

  const tokenize = useCallback(async (): Promise<string> => {
    if (!cardRef.current) throw new Error('Enter your card details first.')
    const result = await cardRef.current.tokenize()
    if (result.status !== 'OK' || !result.token) {
      throw new Error(result.errors?.[0]?.message || 'Card verification failed.')
    }
    return result.token
  }, [])

  const reset = useCallback(() => {
    try {
      cardRef.current?.destroy?.()
    } catch {
      // best-effort teardown
    }
    cardRef.current = null
    setReady(false)
    setError('')
  }, [])

  return { mount, tokenize, reset, ready, mounting, error, setError }
}
