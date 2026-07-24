'use client'

import { Text } from '@hanzo/commerce-ui'
import type { Point } from '@/lib/usage-series'

// Lightweight, dependency-free chart primitives built from the admin's own
// design tokens. Theme-aware via `currentColor` (color set by a `text-*` token).
// No external chart library — the dashboard only needs bars, a sparkline, and a
// horizontal meter.

const IDENTITY = (v: number) => String(v)

function niceMax(max: number): number {
  if (max <= 0) return 1
  const mag = Math.pow(10, Math.floor(Math.log10(max)))
  const norm = max / mag
  const step = norm <= 1 ? 1 : norm <= 2 ? 2 : norm <= 5 ? 5 : 10
  return step * mag
}

/** Pick ~6 evenly-spaced tick indices so x labels never crowd. */
function tickIndices(length: number, target = 6): Set<number> {
  const ticks = new Set<number>()
  if (length === 0) return ticks
  const step = Math.max(1, Math.round(length / target))
  for (let i = 0; i < length; i += step) ticks.add(i)
  ticks.add(length - 1)
  return ticks
}

interface BarChartProps {
  data: Point[]
  format?: (v: number) => string
  /** A `text-*` token — the bar fill inherits it via currentColor. */
  colorClass?: string
  height?: number
  emptyLabel?: string
}

export function BarChart({
  data,
  format = IDENTITY,
  colorClass = 'text-ui-fg-base',
  height = 160,
  emptyLabel = 'No data yet',
}: BarChartProps) {
  const total = data.reduce((s, p) => s + p.value, 0)
  if (data.length === 0 || total === 0) {
    return <EmptyChart height={height} label={emptyLabel} />
  }

  const max = niceMax(Math.max(...data.map((p) => p.value)))
  const ticks = tickIndices(data.length)

  return (
    <div>
      <div className={`flex items-end gap-1 ${colorClass}`} style={{ height }}>
        {data.map((p) => {
          const pct = max > 0 ? (p.value / max) * 100 : 0
          return (
            <div key={p.key} className="flex h-full flex-1 flex-col justify-end">
              <div
                title={`${p.label}: ${format(p.value)}`}
                className="w-full rounded-t-sm bg-current transition-opacity hover:opacity-70"
                style={{ height: `${Math.max(p.value > 0 ? 2 : 0, pct)}%` }}
              />
            </div>
          )
        })}
      </div>
      <div className="mt-2 flex gap-1">
        {data.map((p, i) => (
          <div key={p.key} className="flex-1 text-center">
            {ticks.has(i) ? (
              <Text as="span" size="xsmall" className="text-ui-fg-muted">{p.label}</Text>
            ) : null}
          </div>
        ))}
      </div>
    </div>
  )
}

interface SparklineProps {
  data: Point[]
  format?: (v: number) => string
  colorClass?: string
  height?: number
  emptyLabel?: string
}

export function Sparkline({
  data,
  format = IDENTITY,
  colorClass = 'text-ui-fg-interactive',
  height = 64,
  emptyLabel = 'No data yet',
}: SparklineProps) {
  if (data.length < 2) return <EmptyChart height={height} label={emptyLabel} />

  const values = data.map((p) => p.value)
  const max = Math.max(...values)
  const min = Math.min(...values)
  const span = max - min || 1
  const W = 100
  const H = 100
  const step = W / (data.length - 1)

  const coords = data.map((p, i) => {
    const x = i * step
    const y = H - ((p.value - min) / span) * H
    return [x, y] as const
  })

  const line = coords.map(([x, y], i) => `${i === 0 ? 'M' : 'L'}${x.toFixed(2)},${y.toFixed(2)}`).join(' ')
  const area = `${line} L${W},${H} L0,${H} Z`
  const last = data[data.length - 1]

  return (
    <div className={colorClass}>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        preserveAspectRatio="none"
        style={{ height, width: '100%' }}
        role="img"
        aria-label={`Trend, latest ${format(last.value)}`}
      >
        <path d={area} fill="currentColor" opacity={0.12} />
        <path d={line} fill="none" stroke="currentColor" strokeWidth={2} vectorEffect="non-scaling-stroke" strokeLinejoin="round" strokeLinecap="round" />
      </svg>
    </div>
  )
}

interface MeterProps {
  label: string
  used: number
  total: number
  format?: (v: number) => string
  colorClass?: string
}

/** Horizontal progress meter — used allowance vs total, credit burn-down. */
export function Meter({ label, used, total, format = IDENTITY, colorClass = 'bg-ui-fg-base' }: MeterProps) {
  const pct = total > 0 ? Math.min(100, Math.max(0, (used / total) * 100)) : 0
  const remaining = Math.max(0, total - used)
  return (
    <div>
      <div className="mb-1.5 flex items-baseline justify-between gap-2">
        <Text as="span" size="small" className="text-ui-fg-subtle">{label}</Text>
        <Text as="span" size="small" weight="plus" className="text-ui-fg-base">
          {format(used)} <span className="text-ui-fg-muted">/ {format(total)}</span>
        </Text>
      </div>
      <div className="h-2 w-full overflow-hidden rounded-full bg-ui-bg-component">
        <div className={`h-full rounded-full ${colorClass}`} style={{ width: `${pct}%` }} />
      </div>
      <Text size="xsmall" className="mt-1 text-ui-fg-muted">
        {format(remaining)} remaining
      </Text>
    </div>
  )
}

function EmptyChart({ height, label }: { height: number; label: string }) {
  return (
    <div
      className="flex items-center justify-center rounded-lg border border-dashed border-ui-border-base"
      style={{ height }}
    >
      <Text size="small" className="text-ui-fg-muted">{label}</Text>
    </div>
  )
}
