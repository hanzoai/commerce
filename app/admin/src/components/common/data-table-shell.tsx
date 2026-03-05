'use client'

import { useState, useMemo } from 'react'
import { useRouter } from 'next/navigation'
import { DataTable, useDataTable } from '@hanzo/commerce-ui'
import type {
  DataTableColumnDef,
  DataTablePaginationState,
  DataTableSortingState,
} from '@hanzo/commerce-ui'
import { useList } from '@/lib/api/hooks'
import type { ListParams } from '@/lib/api/data-provider'
import { PageHeader } from './page-header'

interface DataTableShellProps<T> {
  kind: string
  title: string
  description?: string
  columns: DataTableColumnDef<T, any>[]
  detailPath?: string
  getRowId?: (row: T) => string
  actions?: React.ReactNode
}

export function DataTableShell<T>({
  kind,
  title,
  description,
  columns,
  detailPath,
  getRowId = (row: any) => row.id,
  actions,
}: DataTableShellProps<T>) {
  const router = useRouter()
  const [pagination, setPagination] = useState<DataTablePaginationState>({
    pageIndex: 0,
    pageSize: 20,
  })
  const [sorting, setSorting] = useState<DataTableSortingState | null>(null)
  const [search, setSearch] = useState('')

  const params = useMemo<ListParams>(() => {
    const p: ListParams = {
      page: pagination.pageIndex + 1,
      display: pagination.pageSize,
    }
    if (sorting) p.sort = `${sorting.desc ? '-' : ''}${sorting.id}`
    if (search) p.q = search
    return p
  }, [pagination, sorting, search])

  const { data, isLoading } = useList<T>(kind, params)

  const table = useDataTable({
    data: data?.models ?? [],
    columns,
    getRowId,
    rowCount: data?.count ?? 0,
    isLoading,
    pagination: { state: pagination, onPaginationChange: setPagination },
    sorting: { state: sorting, onSortingChange: setSorting },
    search: { state: search, onSearchChange: setSearch },
    onRowClick: detailPath
      ? (_e, row) => router.push(`${detailPath}/${getRowId(row)}`)
      : undefined,
  })

  return (
    <div>
      <PageHeader title={title} description={description} actions={actions} />
      <div className="p-8">
        <DataTable instance={table}>
          <DataTable.Toolbar>
            <DataTable.Search />
            <DataTable.SortingMenu />
          </DataTable.Toolbar>
          <DataTable.Table />
          <DataTable.Pagination />
        </DataTable>
      </div>
    </div>
  )
}
