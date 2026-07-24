'use client'

import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useOrganizations } from '@hanzo/iam/react'
import { useGet } from '@/lib/api/hooks'
import { createOne, updateOne, deleteOne } from '@/lib/api/data-provider'
import {
  formToApplicationMethod,
  formToPromotion,
  type ApplicationMethod,
  type DiscountFormValues,
  type DiscountMetadata,
  type Promotion,
} from './types'

// The list API scopes reads by org; every discount query key is org-prefixed the
// same way the shared hooks do, so this invalidation matches useList/useGet.
const promotionKey = (org: string | null) => [org ?? '__no_org__', 'promotion']

/** The single promotion GET behind a discount — one request, no waterfall. */
export function useDiscount(id: string | undefined) {
  return useGet<Promotion>('promotion', id)
}

/**
 * Create or update a discount in one call. The promotion is the source of truth
 * (its metadata carries the value); the linked `applicationmethod` is upserted
 * best-effort so the backend Evaluate engine actually applies the discount. Its
 * id is stored back in metadata because the list API cannot filter by promotionId.
 */
export function useSaveDiscount() {
  const { currentOrgId } = useOrganizations()
  const qc = useQueryClient()
  const [isPending, setPending] = useState(false)

  const save = async (values: DiscountFormValues, existing?: Promotion): Promise<Promotion> => {
    setPending(true)
    try {
      const body = formToPromotion(values, existing)
      let promo = existing
        ? await updateOne<Promotion>('promotion', existing.id, body, currentOrgId)
        : await createOne<Promotion>('promotion', body, currentOrgId)

      // Projection for the discount engine — never blocks the primary save.
      try {
        const amPayload = formToApplicationMethod(values, promo.id)
        const amId = promo.metadata?.applicationMethodId
        if (amId) {
          await updateOne<ApplicationMethod>('applicationmethod', amId, amPayload, currentOrgId)
        } else {
          const am = await createOne<ApplicationMethod>('applicationmethod', amPayload, currentOrgId)
          const metadata: DiscountMetadata = { ...(promo.metadata ?? {}), applicationMethodId: am.id }
          promo = await updateOne<Promotion>('promotion', promo.id, { metadata }, currentOrgId)
        }
      } catch {
        // The promotion holds the full discount in metadata; the engine projection
        // is a convenience and its failure must not surface as a save error.
      }

      return promo
    } finally {
      setPending(false)
      qc.invalidateQueries({ queryKey: promotionKey(currentOrgId) })
    }
  }

  return { save, isPending }
}

/** Delete a discount (+ its projection) with cache invalidation. */
export function useDeleteDiscount() {
  const { currentOrgId } = useOrganizations()
  const qc = useQueryClient()
  const [isPending, setPending] = useState(false)

  const remove = async (promo: Promotion): Promise<void> => {
    setPending(true)
    try {
      await deleteOne('promotion', promo.id, currentOrgId)
      const amId = promo.metadata?.applicationMethodId
      if (amId) {
        try {
          await deleteOne('applicationmethod', amId, currentOrgId)
        } catch {
          // Projection cleanup is best-effort.
        }
      }
    } finally {
      setPending(false)
      qc.invalidateQueries({ queryKey: promotionKey(currentOrgId) })
    }
  }

  return { remove, isPending }
}
