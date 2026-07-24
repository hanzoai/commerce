// Tiny cross-component AI-dock bus. Lets any surface (e.g. the onboarding wizard's
// "generate my catalog" shortcut) open the AI dock pre-filled with a prompt without
// URL params — which would force Suspense boundaries under `output: 'export'`.
//
// Mirrors search-bus: a same-route listener fires immediately; a cross-route caller
// stashes `pending` then navigates, and the dock consumes it on mount.
type Listener = (prompt: string) => void

let pending: string | null = null
const listeners = new Set<Listener>()

export function requestAiPrompt(prompt: string): void {
  pending = prompt
  listeners.forEach((fn) => fn(prompt))
}

export function consumePendingAiPrompt(): string | null {
  const prompt = pending
  pending = null
  return prompt
}

export function onAiPrompt(fn: Listener): () => void {
  listeners.add(fn)
  return () => {
    listeners.delete(fn)
  }
}
