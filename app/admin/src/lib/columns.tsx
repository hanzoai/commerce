'use client'

/**
 * The renderer half of the catalog: a `ColumnSpec` → a `@hanzo/ui/product` `Column`.
 *
 * Seven cell shapes cover every merchant list, so a resource declares WHAT a column
 * is and this file decides HOW it reads. Money is stored in minor units by commerce
 * and formatted in the row's own currency; a missing value is an em dash, never a
 * fabricated zero.
 */
import type { ReactNode } from 'react'
import { Text } from '@hanzo/gui'
import { StatusTag, type Column } from '@hanzo/ui/product'

import type { ColumnSpec, Row } from './resources'

const str = (r: Row, k: string): string => (r[k] == null ? '' : String(r[k]))

const Muted = ({ children }: { children: ReactNode }) => (
  <Text fontSize="$3" color="$color11" numberOfLines={1}>
    {children}
  </Text>
)
const Strong = ({ children }: { children: ReactNode }) => (
  <Text fontSize="$3" fontWeight="600" color="$color12" numberOfLines={1}>
    {children}
  </Text>
)
const Dash = () => (
  <Text fontSize="$3" color="$color10">
    —
  </Text>
)

const money = (r: Row, k: string): ReactNode => {
  const v = r[k]
  if (typeof v !== 'number') return <Dash />
  const currency = str(r, 'currency') || str(r, 'currencyCode') || 'USD'
  return <Muted>{new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(v / 100)}</Muted>
}

export function toColumn(c: ColumnSpec): Column<Row> {
  switch (c.as) {
    case 'name':
      return { key: c.key, header: c.header, sortable: true, render: (r) => (str(r, c.key) ? <Strong>{str(r, c.key)}</Strong> : <Dash />) }
    case 'date':
      return { key: c.key, header: c.header, width: 120, mono: true, align: 'right', render: (r) => (r[c.key] ? <Muted>{new Date(str(r, c.key)).toLocaleDateString()}</Muted> : <Dash />) }
    case 'money':
      return { key: c.key, header: c.header, width: 120, mono: true, align: 'right', render: (r) => money(r, c.key) }
    case 'num':
      return { key: c.key, header: c.header, width: 100, mono: true, align: 'right', render: (r) => (r[c.key] == null ? <Dash /> : <Muted>{str(r, c.key)}</Muted>) }
    case 'status':
      return { key: c.key, header: c.header, width: 120, render: (r) => (str(r, c.key) ? <StatusTag status={str(r, c.key)} /> : <Dash />) }
    case 'flag':
      return { key: c.key, header: c.header, width: 120, render: (r) => <StatusTag status={r[c.key] ? c.on! : c.off!} /> }
    default:
      return { key: c.key, header: c.header, render: (r) => (str(r, c.key) ? <Muted>{str(r, c.key)}</Muted> : <Dash />) }
  }
}
