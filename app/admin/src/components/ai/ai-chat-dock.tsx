'use client'

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useRouter, usePathname } from 'next/navigation'
import { Button, Heading, Text, Textarea, clx } from '@hanzo/commerce-ui'
import { useIam, useOrganizations } from '@hanzo/iam/react'
import { createOne, fetchCount, fetchList, fetchCurrentStore, fetchModels, getAccessToken } from '@/lib/api/data-provider'
import type { CurrentStore } from '@/lib/api/data-provider'
import { requestSearch } from '@/lib/search-bus'
import { consumePendingAiPrompt, onAiPrompt } from '@/lib/ai-bus'
import {
  SECTION_TO_KIND,
  buildSystemPrompt,
  dispatch,
  parseAssistant,
  type AppHost,
  type CommandLogEntry,
  type Created,
  type ProductSpec,
} from './app-commands'

const AI_URL = process.env.NEXT_PUBLIC_HANZO_AI_URL || 'https://api.hanzo.ai/v1/chat/completions'
const AI_MODEL = process.env.NEXT_PUBLIC_HANZO_AI_MODEL || 'best'

interface ChatMsg {
  role: 'user' | 'assistant'
  content: string
  receipts?: CommandLogEntry[]
}

function sectionOf(pathname: string): string {
  const seg = pathname.split('/').filter(Boolean)[0]
  return seg || 'overview'
}

function slugify(value: string): string {
  return value.toLowerCase().trim().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
}

// The wire shape POSTed to /v1/product — mirrors the CreateProduct admin form
// (name/slug/sku/description/available/hidden) plus optional cents price. ONE
// builder so create_product and generate_catalog derive identical records.
function productPayload(spec: ProductSpec) {
  const name = spec.name.trim()
  const slug = slugify(name)
  const status = spec.status ?? 'draft'
  const price =
    spec.priceUsd != null && Number.isFinite(spec.priceUsd) && spec.priceUsd >= 0
      ? Math.round(spec.priceUsd * 100)
      : undefined
  return {
    name,
    slug,
    sku: spec.sku?.trim() || slug.toUpperCase() || name,
    description: spec.description?.trim(),
    ...(price != null ? { price } : {}),
    available: status === 'live',
    hidden: status === 'hidden',
  }
}

type CreatedRecord = { id?: string; name?: string }

export function AiChatDock() {
  const { accessToken, isAuthenticated } = useIam()
  const { currentOrgId } = useOrganizations()
  const router = useRouter()
  const pathname = usePathname()

  const [open, setOpen] = useState(false)
  const [input, setInput] = useState('')
  const [busy, setBusy] = useState(false)
  const [messages, setMessages] = useState<ChatMsg[]>([])
  const [store, setStore] = useState<CurrentStore | null>(null)
  const scrollRef = useRef<HTMLDivElement>(null)

  // Load the live store once (grounding context) when the dock first opens.
  useEffect(() => {
    if (open && !store) fetchCurrentStore(currentOrgId).then(setStore).catch(() => {})
  }, [open, store, currentOrgId])

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' })
  }, [messages, busy])

  // Open the dock pre-filled when another surface (onboarding "generate my
  // catalog" shortcut) requests a prompt — same-route via the listener,
  // cross-route via the pending stash consumed on mount.
  useEffect(() => {
    const apply = (prompt: string) => {
      setOpen(true)
      setInput(prompt)
    }
    const queued = consumePendingAiPrompt()
    if (queued) apply(queued)
    return onAiPrompt(apply)
  }, [])

  const host: AppHost = useMemo(
    () => ({
      isAuthed: () => !!isAuthenticated,
      currentSection: () => sectionOf(pathname),
      navigate: (section) => router.push(`/${section}`),
      filter: (section, query) => {
        const kind = SECTION_TO_KIND[section] ?? 'product'
        requestSearch(kind, query)
        router.push(`/${section}`)
      },
      summarize: async (section) => {
        if (section === 'models') {
          const models = await fetchModels(currentOrgId)
          const premium = models.filter((m) => m.premium).length
          const top = [...models].sort((a, b) => (b.pricing?.input ?? 0) - (a.pricing?.input ?? 0)).slice(0, 3)
          const list = top.map((m) => `${m.id} ($${(m.pricing?.input ?? 0).toFixed(2)}/$${(m.pricing?.output ?? 0).toFixed(2)} per 1M)`).join(', ')
          return `${models.length} models (${premium} premium). Highest input price: ${list || '—'}.`
        }
        if (SECTION_TO_KIND[section]) {
          const kind = SECTION_TO_KIND[section]
          const res = await fetchList<{ name?: string; email?: string; id: string }>(kind, { display: 5 }, currentOrgId)
          const sample = res.models
            .map((m) => m.name || m.email || m.id)
            .filter(Boolean)
            .slice(0, 5)
            .join(', ')
          return `${section}: ${res.count} total${sample ? `. Sample: ${sample}` : ''}`
        }
        const [products, orders, customers, collections] = await Promise.all([
          fetchCount('product', currentOrgId),
          fetchCount('order', currentOrgId),
          fetchCount('c/user', currentOrgId),
          fetchCount('collection', currentOrgId),
        ])
        const listings = store?.listings ? Object.keys(store.listings).length : 0
        return `Products: ${products}, Orders: ${orders}, Customers: ${customers}, Collections: ${collections}. Store ${store?.name ?? '—'}: ${listings} listings.`
      },
      createProduct: async (spec) => {
        const payload = productPayload(spec)
        const product = await createOne<CreatedRecord>('product', payload, currentOrgId)
        router.push('/products')
        return { id: product.id, name: product.name || payload.name }
      },
      createCollection: async ({ title, handle }) => {
        const name = title.trim()
        const collection = await createOne<CreatedRecord & { slug: string }>(
          'collection',
          { name, slug: handle?.trim() || slugify(name) },
          currentOrgId,
        )
        router.push('/collections')
        return { id: collection.id, title: collection.name || name }
      },
      createStore: async ({ name, currency }) => {
        const clean = name.trim()
        const store = await createOne<CreatedRecord & { currency: string }>(
          'store',
          { name: clean, currency: (currency?.trim() || 'usd').toLowerCase() },
          currentOrgId,
        )
        router.push('/settings')
        return { id: store.id, name: store.name || clean }
      },
      generateCatalog: async (theme, count, specs) => {
        const list: ProductSpec[] = specs.length
          ? specs.slice(0, count)
          : Array.from({ length: count }, (_, i) => ({ name: `${theme} ${i + 1}` }))
        const created: Created[] = []
        let failed = 0
        for (const spec of list) {
          try {
            const payload = productPayload(spec)
            const product = await createOne<CreatedRecord>('product', payload, currentOrgId)
            created.push({ id: product.id, name: product.name || payload.name })
          } catch {
            failed += 1
          }
        }
        if (created.length) router.push('/products')
        return { created, failed }
      },
    }),
    [isAuthenticated, pathname, router, currentOrgId, store],
  )

  const liveContext = useCallback(() => {
    const parts = [`Current section: ${sectionOf(pathname)}.`]
    if (store) {
      const listings = store.listings ? Object.keys(store.listings).length : 0
      parts.push(`Store: ${store.name} (${store.domain ?? store.slug}, ${(store.currency ?? 'usd').toUpperCase()}), ${listings} listings.`)
    }
    return parts.join(' ')
  }, [pathname, store])

  const send = useCallback(async () => {
    const text = input.trim()
    if (!text || busy) return

    // The dashboard layout sets the shared token; prefer it over the hook value,
    // which can still be null on the first render after the dock opens. Without a
    // bearer the chat endpoint 401s and the reply reads as a blanket "unavailable".
    const token = accessToken ?? getAccessToken()
    if (!token) {
      setMessages((m) => [...m, { role: 'assistant', content: 'Sign in to use the assistant.' }])
      return
    }

    setInput('')
    const nextMsgs: ChatMsg[] = [...messages, { role: 'user', content: text }]
    setMessages(nextMsgs)
    setBusy(true)
    try {
      const history = nextMsgs.slice(-10).map((m) => ({ role: m.role, content: m.content }))
      const body = {
        model: AI_MODEL,
        stream: false,
        temperature: 0.3,
        messages: [{ role: 'system', content: buildSystemPrompt(liveContext()) }, ...history],
      }
      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      }
      if (currentOrgId) headers['X-Org-Id'] = currentOrgId
      const res = await fetch(AI_URL, { method: 'POST', headers, body: JSON.stringify(body) })
      if (!res.ok) {
        // Surface the REAL failure in dev instead of masking it behind a blanket
        // message — status + body are what actually diagnose a 401/402/5xx.
        const detail = await res.text().catch(() => '')
        if (process.env.NODE_ENV !== 'production') {
          console.error(`[ai-dock] ${AI_URL} ${res.status} ${res.statusText}`, detail)
        }
        const message =
          res.status === 401 || res.status === 403
            ? 'Your session expired. Refresh the page and sign in again.'
            : res.status === 402
              ? 'The assistant needs an active plan on this store.'
              : `The assistant hit an error (${res.status}). Please try again.`
        setMessages((m) => [...m, { role: 'assistant', content: message }])
        return
      }
      const data = await res.json()
      const content: string = data?.choices?.[0]?.message?.content ?? ''
      const { reply, actions } = parseAssistant(content)
      const receipts = await dispatch(actions, host)
      setMessages((m) => [...m, { role: 'assistant', content: reply || '(no response)', receipts: receipts.length ? receipts : undefined }])
    } catch (err) {
      if (process.env.NODE_ENV !== 'production') {
        console.error('[ai-dock] request failed', err)
      }
      setMessages((m) => [...m, { role: 'assistant', content: 'Could not reach the assistant. Check your connection and try again.' }])
    } finally {
      setBusy(false)
    }
  }, [input, busy, messages, liveContext, accessToken, currentOrgId, host])

  return (
    <>
      {/* Floating action button */}
      <button
        type="button"
        aria-label={open ? 'Close assistant' : 'Open assistant'}
        onClick={() => setOpen((o) => !o)}
        className={clx(
          'fixed bottom-5 right-5 z-50 flex h-12 w-12 items-center justify-center rounded-full',
          'bg-ui-button-inverted text-ui-fg-on-inverted shadow-elevation-flyout transition-transform hover:scale-105',
        )}
      >
        {open ? <IconX /> : <IconSparkle />}
      </button>

      {/* Right-side dock */}
      <aside
        className={clx(
          'fixed inset-y-0 right-0 z-40 flex w-full max-w-[400px] flex-col border-l border-ui-border-base bg-ui-bg-base shadow-elevation-flyout',
          'transition-transform duration-200 ease-out',
          open ? 'translate-x-0' : 'pointer-events-none translate-x-full',
        )}
        aria-hidden={!open}
      >
        <header className="flex items-center justify-between border-b border-ui-border-base px-4 py-3">
          <div>
            <Heading level="h3">Assistant</Heading>
            <Text size="xsmall" className="text-ui-fg-muted">{store?.name ? `${store.name} · ` : ''}Ask, create, navigate</Text>
          </div>
          <button type="button" aria-label="Close" onClick={() => setOpen(false)} className="rounded-md p-1 text-ui-fg-muted hover:bg-ui-bg-base-hover">
            <IconX />
          </button>
        </header>

        <div ref={scrollRef} className="flex-1 space-y-3 overflow-y-auto px-4 py-4">
          {messages.length === 0 ? (
            <div className="mt-8 text-center">
              <Text size="small" className="text-ui-fg-muted">
                Try “add a product called Cold Brew for $6”, “generate a 6-item coffee catalog”, or “summarize this view”.
              </Text>
            </div>
          ) : (
            messages.map((m, i) => <Bubble key={i} msg={m} onOpen={(href) => router.push(href)} />)
          )}
          {busy && (
            <div className="flex gap-1 px-1 py-2">
              <Dot /> <Dot /> <Dot />
            </div>
          )}
        </div>

        <div className="border-t border-ui-border-base p-3">
          <div className="flex items-end gap-2">
            <Textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault()
                  send()
                }
              }}
              placeholder={isAuthenticated ? 'Message the assistant…' : 'Sign in to chat'}
              disabled={!isAuthenticated || busy}
              rows={1}
              className="max-h-32 min-h-[40px] flex-1 resize-none"
            />
            <Button variant="primary" size="small" onClick={send} disabled={!input.trim() || busy || !isAuthenticated}>
              Send
            </Button>
          </div>
        </div>
      </aside>
    </>
  )
}

function Bubble({ msg, onOpen }: { msg: ChatMsg; onOpen: (href: string) => void }) {
  const isUser = msg.role === 'user'
  return (
    <div className={clx('flex', isUser ? 'justify-end' : 'justify-start')}>
      <div
        className={clx(
          'max-w-[85%] rounded-lg px-3 py-2',
          isUser ? 'bg-ui-button-inverted text-ui-fg-on-inverted' : 'bg-ui-bg-component text-ui-fg-base',
        )}
      >
        <Text size="small" className={clx('whitespace-pre-wrap', isUser ? 'text-ui-fg-on-inverted' : 'text-ui-fg-base')}>
          {msg.content}
        </Text>
        {msg.receipts?.length ? (
          <div className="mt-2 space-y-1 border-t border-ui-border-base pt-2">
            {msg.receipts.map((r, i) => (
              <Text key={i} size="xsmall" className={clx('block', r.ok ? 'text-ui-fg-subtle' : 'text-ui-fg-error')}>
                {r.ok ? '✓' : '✗'} {r.message}
                {r.ok && r.href ? (
                  <button
                    type="button"
                    onClick={() => onOpen(r.href!)}
                    className="ml-1 underline underline-offset-2 hover:text-ui-fg-base"
                  >
                    Open
                  </button>
                ) : null}
              </Text>
            ))}
          </div>
        ) : null}
      </div>
    </div>
  )
}

function Dot() {
  return <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-ui-fg-muted" />
}

function IconSparkle() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 3l1.9 4.6L18.5 9.5 13.9 11.4 12 16l-1.9-4.6L5.5 9.5l4.6-1.9L12 3z" />
      <path d="M19 15l.8 2 2 .8-2 .8-.8 2-.8-2-2-.8 2-.8.8-2z" />
    </svg>
  )
}

function IconX() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M18 6L6 18M6 6l12 12" />
    </svg>
  )
}
