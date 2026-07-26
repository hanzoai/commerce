'use client'

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useOrganizations } from '@hanzo/iam/react'
import { fetchList, fetchOne, createOne, updateOne, deleteOne, fetchCount, fetchCurrentStore, fetchModels, fetchIntegrations, saveIntegration, setStoreId, fetchAction, postAction, uploadImages } from '../data-provider'
import type { ListParams, ListResponse, CurrentStore, HanzoModel, CommerceIntegration, IntegrationInput } from '../data-provider'

/** Every query key is prefixed with the current org so switching orgs gives a clean cache. */
function orgKey(org: string | null, kind: string, ...rest: unknown[]) {
  return [org ?? '__no_org__', kind, ...rest]
}

export function useList<T>(kind: string, params?: ListParams) {
  const { currentOrgId } = useOrganizations()
  return useQuery<ListResponse<T>>({
    queryKey: orgKey(currentOrgId, kind, 'list', params),
    queryFn: () => fetchList<T>(kind, params, currentOrgId),
  })
}

export function useGet<T>(kind: string, id: string | undefined) {
  const { currentOrgId } = useOrganizations()
  return useQuery<T>({
    queryKey: orgKey(currentOrgId, kind, 'detail', id),
    queryFn: () => fetchOne<T>(kind, id!, currentOrgId),
    enabled: !!id,
  })
}

export function useCreate<T>(kind: string) {
  const { currentOrgId } = useOrganizations()
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: Partial<T>) => createOne<T>(kind, data, currentOrgId),
    onSuccess: () => qc.invalidateQueries({ queryKey: orgKey(currentOrgId, kind) }),
  })
}

export function useUpdate<T>(kind: string) {
  const { currentOrgId } = useOrganizations()
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<T> }) => updateOne<T>(kind, id, data, currentOrgId),
    onSuccess: () => qc.invalidateQueries({ queryKey: orgKey(currentOrgId, kind) }),
  })
}

export function useDelete(kind: string) {
  const { currentOrgId } = useOrganizations()
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deleteOne(kind, id, currentOrgId),
    onSuccess: () => qc.invalidateQueries({ queryKey: orgKey(currentOrgId, kind) }),
  })
}

export function useCount(kind: string) {
  const { currentOrgId } = useOrganizations()
  return useQuery<number>({
    queryKey: orgKey(currentOrgId, kind, 'count'),
    queryFn: () => fetchCount(kind, currentOrgId),
  })
}

/** Read a resource action sub-route (GET /v1/{kind}/{id}/{action}), org-scoped.
 *  Used for projections like a gift card's live balance + redemption ledger. The
 *  key is a child of orgKey(org, kind), so any create/update/action invalidation
 *  of the kind refreshes it too. */
export function useResourceActionData<T>(kind: string, id: string | undefined, action: string, options?: { enabled?: boolean }) {
  const { currentOrgId } = useOrganizations()
  return useQuery<T>({
    queryKey: orgKey(currentOrgId, kind, 'action', id, action),
    queryFn: () => fetchAction<T>(kind, id!, action, currentOrgId),
    enabled: !!id && (options?.enabled ?? true),
  })
}

/** Invoke a resource action (POST /v1/{kind}/{id}/{action}) — e.g. redeem/void a
 *  gift card. On success every query for the kind (list, detail, balance,
 *  redemptions) is invalidated so the projected balance re-reads. */
export function useResourceAction<TResult, TBody = unknown>(kind: string, id: string | undefined, action: string) {
  const { currentOrgId } = useOrganizations()
  const qc = useQueryClient()
  return useMutation<TResult, Error, TBody>({
    mutationFn: (body: TBody) => postAction<TResult>(kind, id!, action, body, currentOrgId),
    onSuccess: () => qc.invalidateQueries({ queryKey: orgKey(currentOrgId, kind) }),
  })
}

/** Upload image file(s) to the caller-org's tenant prefix (POST /v1/upload/images).
 *  Returns the public CDN URL(s). Org-scoped; used by the <ImageUpload> control. */
export function useUploadImages() {
  const { currentOrgId } = useOrganizations()
  return useMutation<string[], Error, File[]>({
    mutationFn: (files: File[]) => uploadImages(files, currentOrgId),
  })
}

// Named resource hooks
export const useProducts = (params?: ListParams) => useList<any>('product', params)
export const useProduct = (id?: string) => useGet<any>('product', id)
export const useOrders = (params?: ListParams) => useList<any>('order', params)
export const useOrder = (id?: string) => useGet<any>('order', id)
export const useCustomers = (params?: ListParams) => useList<any>('c/user', params)
export const useCustomer = (id?: string) => useGet<any>('c/user', id)
export const useCollections = (params?: ListParams) => useList<any>('collection', params)
export const useVariants = (params?: ListParams) => useList<any>('variant', params)
export const useStockLocations = (params?: ListParams) => useList<any>('stocklocation', params)

/** The caller-org's live store (GET /v1/store/current — meter-exempt), unwrapped + graceful. */
export function useStore() {
  const { currentOrgId } = useOrganizations()
  return useQuery<CurrentStore | null>({
    queryKey: orgKey(currentOrgId, 'store', 'current'),
    queryFn: () => fetchCurrentStore(currentOrgId),
  })
}

export function useStores() {
  const { currentOrgId } = useOrganizations()
  const qc = useQueryClient()
  const queryKey = orgKey(currentOrgId, 'store', 'list')
  const query = useQuery({
    queryKey,
    queryFn: () => fetchList<CurrentStore>('store', { display: 100 }, currentOrgId),
  })
  const create = useMutation({
    mutationFn: (input: Partial<CurrentStore>) => createOne<CurrentStore>('store', input, currentOrgId),
    onSuccess: async (store) => {
      setStoreId(store.id)
      await qc.invalidateQueries({ queryKey: [currentOrgId || 'no-org'] })
    },
  })
  const select = async (storeId: string) => {
    setStoreId(storeId)
    await qc.invalidateQueries({ queryKey: [currentOrgId || 'no-org'] })
  }
  return { ...query, create, select }
}

/** The org's model catalog (GET /v1/models) with per-model pricing. Org-scoped.
 *  Cached 5min — the catalog changes rarely, and this dedupes the overview + models
 *  page reads into one request (avoids the gateway's per-window rate limit). */
export function useModels() {
  const { currentOrgId } = useOrganizations()
  return useQuery<HanzoModel[]>({
    queryKey: orgKey(currentOrgId, 'models', 'list'),
    queryFn: () => fetchModels(currentOrgId),
    staleTime: 5 * 60 * 1000,
  })
}

export function useIntegrations() {
  const { currentOrgId } = useOrganizations()
  const qc = useQueryClient()
  const queryKey = orgKey(currentOrgId, 'integrations', 'list')
  const query = useQuery({
    queryKey,
    queryFn: () => fetchIntegrations(currentOrgId),
  })
  // One mutation for connect · configure · enable · pause. `data` (creds) is
  // optional — omit it to just flip `enabled`, include it to (re)sync KMS.
  const save = useMutation({
    mutationFn: (input: IntegrationInput) => saveIntegration(input, currentOrgId),
    onSuccess: integrations => qc.setQueryData(queryKey, integrations),
  })
  return { ...query, save }
}
