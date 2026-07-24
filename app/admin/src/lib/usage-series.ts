// Pure time-series aggregation for the usage/analytics dashboard. No I/O, no
// React — bucket a list of dated records into a continuous day series (gaps
// filled with 0) so charts render a stable axis regardless of sparse data.

export interface Point {
  /** ISO day key (YYYY-MM-DD), stable + sortable. */
  key: string
  /** Short axis label (e.g. "Jul 3"). */
  label: string
  value: number
}

function dayKey(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function dayLabel(d: Date): string {
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
}

/**
 * Sum `getValue` over the last `days` calendar days, bucketed per day. Records
 * with an unparseable/out-of-window date are ignored. The returned series is
 * always exactly `days` long, oldest → newest, with missing days at 0.
 */
export function bucketByDay<T>(
  items: T[],
  getTime: (item: T) => string | number | Date | undefined | null,
  getValue: (item: T) => number,
  days = 14,
): Point[] {
  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())

  const series: Point[] = []
  const index = new Map<string, number>()
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date(today)
    d.setDate(today.getDate() - i)
    const key = dayKey(d)
    index.set(key, series.length)
    series.push({ key, label: dayLabel(d), value: 0 })
  }

  for (const item of items) {
    const raw = getTime(item)
    if (raw == null) continue
    const d = new Date(raw)
    if (Number.isNaN(d.getTime())) continue
    const key = dayKey(new Date(d.getFullYear(), d.getMonth(), d.getDate()))
    const at = index.get(key)
    if (at == null) continue
    series[at].value += getValue(item) || 0
  }

  return series
}

/** Running cumulative total of a series (for burn-down / growth curves). */
export function cumulative(points: Point[]): Point[] {
  let sum = 0
  return points.map((p) => {
    sum += p.value
    return { ...p, value: sum }
  })
}

/**
 * Descending remaining-balance curve: starting from `start`, subtract each
 * day's spend. Never drops below 0. Used for the credit burn-down view.
 */
export function burnDown(points: Point[], start: number): Point[] {
  let remaining = start
  return points.map((p) => {
    remaining = Math.max(0, remaining - p.value)
    return { ...p, value: remaining }
  })
}

/** Sum of a numeric field across a list (small helper, avoids inline reduces). */
export function sumBy<T>(items: T[], getValue: (item: T) => number): number {
  let total = 0
  for (const item of items) total += getValue(item) || 0
  return total
}

/**
 * Group records by a string key, summing a value and counting rows. Sorted by
 * value descending. Used for the per-model AI spend breakdown.
 */
export interface Group {
  key: string
  value: number
  count: number
}

export function groupBy<T>(
  items: T[],
  getKey: (item: T) => string | undefined | null,
  getValue: (item: T) => number,
): Group[] {
  const map = new Map<string, Group>()
  for (const item of items) {
    const key = (getKey(item) || 'unknown').trim() || 'unknown'
    const value = getValue(item) || 0
    const existing = map.get(key)
    if (existing) {
      existing.value += value
      existing.count += 1
    } else {
      map.set(key, { key, value, count: 1 })
    }
  }
  return Array.from(map.values()).sort((a, b) => b.value - a.value)
}
