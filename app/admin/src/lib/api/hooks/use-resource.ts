'use client'

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useOrganizations } from '@hanzo/iam/react'
import { fetchList, fetchOne, createOne, updateOne, deleteOne, fetchCount } from '../data-provider'
import type { ListParams, ListResponse } from '../data-provider'

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
export const useStore = () => useGet<any>('store', 'current')
