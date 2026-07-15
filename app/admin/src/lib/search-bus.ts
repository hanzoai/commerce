// Tiny cross-component search bus. Lets the AI dock drive a list section's existing
// search state (the same `q` the DataTable search box sets) without URL params —
// which would force Suspense boundaries under `output: 'export'`.
//
// Keyed by data-provider `kind` (product, order, c/user, collection, stocklocation).
// Cross-route: the dock sets `pending[kind]` then navigates; the target DataTableShell
// consumes it on mount. Same-route: listeners fire immediately.
type Listener = (q: string) => void

const pending: Record<string, string> = {}
const listeners: Record<string, Set<Listener>> = {}

export function requestSearch(kind: string, q: string): void {
  pending[kind] = q
  listeners[kind]?.forEach((fn) => fn(q))
}

export function consumePending(kind: string): string {
  const q = pending[kind] ?? ''
  delete pending[kind]
  return q
}

export function onSearch(kind: string, fn: Listener): () => void {
  ;(listeners[kind] ??= new Set()).add(fn)
  return () => {
    listeners[kind]?.delete(fn)
  }
}
