// Shared zod field helpers — the one place form field validation is defined, so
// customer + group forms compose from the same primitives instead of re-declaring.
import { z } from 'zod'

/** Required, whitespace-trimmed, non-empty text. */
export const requiredText = z.string().trim().min(1, 'Required')

/** Optional text — empty string is allowed and treated as "unset". */
export const optionalText = z.string().trim().optional()

/** A valid email address (required). */
export const emailField = z.string().trim().min(1, 'Required').email('Enter a valid email')

/** Strip '' / null / undefined values so a PATCH/POST only carries real edits. */
export function cleanEmpty<T extends Record<string, unknown>>(values: T): Partial<T> {
  const out: Record<string, unknown> = {}
  for (const [k, v] of Object.entries(values)) {
    if (v === undefined || v === null) continue
    if (typeof v === 'string' && v.trim() === '') continue
    out[k] = v
  }
  return out as Partial<T>
}

/** One consistent human message from an unknown thrown value. */
export function errorMessage(e: unknown, fallback = 'Something went wrong'): string {
  return e instanceof Error && e.message ? e.message : fallback
}
