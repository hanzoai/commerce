'use client'

import { useCallback, useEffect, useState } from 'react'

// Client-side onboarding progress: the resume step + a completed flag, persisted
// to localStorage so a reload lands the merchant back where they left off. No URL
// params (which force Suspense under `output: 'export'`).
const KEY = 'hanzo.commerce.onboarding'

interface Persisted {
  step: number
  done: boolean
}

function read(): Persisted {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return { step: 0, done: false }
    const parsed = JSON.parse(raw) as Partial<Persisted>
    return { step: Number(parsed.step) || 0, done: !!parsed.done }
  } catch {
    return { step: 0, done: false }
  }
}

export function useOnboarding(count: number) {
  const [step, setStep] = useState(0)
  const [done, setDone] = useState(false)
  // `ready` gates first paint on the hydrated value so we never flash step 1.
  const [ready, setReady] = useState(false)

  useEffect(() => {
    const s = read()
    setStep(Math.max(0, Math.min(count - 1, s.step)))
    setDone(s.done)
    setReady(true)
  }, [count])

  const save = useCallback((nextStep: number, nextDone: boolean) => {
    try {
      localStorage.setItem(KEY, JSON.stringify({ step: nextStep, done: nextDone }))
    } catch {
      // Private mode / disabled storage — progress just isn't persisted.
    }
  }, [])

  const goTo = useCallback(
    (next: number) => {
      const clamped = Math.max(0, Math.min(count - 1, next))
      setStep(clamped)
      setDone(false)
      save(clamped, false)
    },
    [count, save],
  )

  const finish = useCallback(() => {
    setDone(true)
    setStep((cur) => {
      save(cur, true)
      return cur
    })
  }, [save])

  const reset = useCallback(() => {
    setStep(0)
    setDone(false)
    save(0, false)
  }, [save])

  return { step, done, ready, goTo, finish, reset }
}
