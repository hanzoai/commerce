'use client'

// The per-location stock levels of one inventory item, read from
// /v1/inventorylevel filtered by inventoryItemId. Shown below the general fields
// on the inventory-item detail surface via the generic <ResourceEdit> `extra` slot.
import { Text } from '@hanzo/commerce-ui'
import { Section } from '@/components/common/detail-view/section'
import { SectionRow } from '@/components/common/section/section-row'
import { useList } from '@/lib/api/hooks'
import { inventoryLevelKind, type InventoryLevel } from '@/lib/inventory-item'

export function InventoryLevels({ id }: { id: string }) {
  const { data, isLoading } = useList<InventoryLevel>(inventoryLevelKind, { inventoryItemId: id, display: 100 })
  const levels = data?.models ?? []

  return (
    <Section title="Stock levels">
      {isLoading ? (
        <div className="px-6 py-4">
          <Text size="small" className="text-ui-fg-muted">Loading…</Text>
        </div>
      ) : levels.length === 0 ? (
        <div className="px-6 py-4">
          <Text size="small" className="text-ui-fg-muted">No stock tracked at any location.</Text>
        </div>
      ) : (
        <div className="divide-y">
          {levels.map((l) => (
            <SectionRow
              key={l.id}
              title={l.locationId}
              value={`${l.stockedQuantity ?? 0} stocked · ${l.reservedQuantity ?? 0} reserved · ${l.incomingQuantity ?? 0} incoming`}
            />
          ))}
        </div>
      )}
    </Section>
  )
}
