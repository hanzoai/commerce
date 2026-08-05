'use client'

/**
 * ⌘K — every surface in the admin, reachable by typing its name.
 *
 * The catalog already knows what exists (`RESOURCES` + Overview), so the palette
 * is a VIEW of it rather than a second list to keep in step: a resource added to
 * the catalog is in the palette and the sidebar at the same moment, and one that
 * is removed cannot linger in either.
 *
 * It is deliberately plain DOM. @hanzo/gui 7.3 ships no command or dialog
 * primitive, and inventing a half one here would be a component the design
 * system then has to absorb; this is a listbox and an input, styled with the
 * same tokens as everything around it.
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useRouter } from 'next/navigation'

export type Surface = { slug: string; label: string }

/** Sub-sequence match — "prol" finds "price-lists", which prefix matching cannot. */
function matches(haystack: string, needle: string): boolean {
  if (!needle) return true
  let i = 0
  for (const ch of haystack.toLowerCase()) {
    if (ch === needle[i]) i++
    if (i === needle.length) return true
  }
  return false
}

/**
 * Open the palette from anywhere.
 *
 * On a narrow screen the palette IS the navigation — the sidebar is 232px of a
 * 390px viewport, so it steps aside and this takes over. A keystroke cannot be
 * typed on a touch device, so the palette needs a door that is not ⌘K; this is
 * that door, and it stays a plain event so nothing has to thread a setter down
 * through the shell.
 */
const OPEN = 'hanzo.admin.palette.open'

export function openPalette() {
  window.dispatchEvent(new Event(OPEN))
}

export function Palette({ surfaces }: { surfaces: Surface[] }) {
  const router = useRouter()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [cursor, setCursor] = useState(0)
  const input = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const show = () => setOpen(true)
    window.addEventListener(OPEN, show)
    return () => window.removeEventListener(OPEN, show)
  }, [])

  const hits = useMemo(() => {
    const q = query.trim().toLowerCase()
    return surfaces.filter((s) => matches(s.label, q) || matches(s.slug, q))
  }, [surfaces, query])

  // ⌘K opens, and it TOGGLES: the same keystroke that summoned it dismisses it,
  // which is what everyone's hands already expect.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setOpen((v) => !v)
      } else if (e.key === 'Escape') {
        setOpen(false)
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [])

  useEffect(() => {
    if (!open) return
    setQuery('')
    setCursor(0)
    input.current?.focus()
  }, [open])

  const go = useCallback(
    (slug: string) => {
      setOpen(false)
      router.push(`/${slug}`)
    },
    [router],
  )

  if (!open) return null

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Go to"
      onMouseDown={() => setOpen(false)}
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 60,
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'flex-start',
        paddingTop: '14vh',
        background: 'rgba(0,0,0,0.6)',
        backdropFilter: 'blur(2px)',
      }}
    >
      <div
        onMouseDown={(e) => e.stopPropagation()}
        style={{
          width: 'min(560px, calc(100vw - 32px))',
          background: '#0a0a0a',
          border: '1px solid rgba(255,255,255,0.12)',
          borderRadius: 12,
          overflow: 'hidden',
          boxShadow: '0 24px 64px rgba(0,0,0,0.6)',
        }}
      >
        <input
          ref={input}
          value={query}
          onChange={(e) => {
            setQuery(e.target.value)
            setCursor(0)
          }}
          onKeyDown={(e) => {
            if (e.key === 'ArrowDown') {
              e.preventDefault()
              setCursor((c) => Math.min(c + 1, hits.length - 1))
            } else if (e.key === 'ArrowUp') {
              e.preventDefault()
              setCursor((c) => Math.max(c - 1, 0))
            } else if (e.key === 'Enter' && hits[cursor]) {
              e.preventDefault()
              go(hits[cursor].slug)
            }
          }}
          placeholder="Go to…"
          aria-label="Go to"
          style={{
            width: '100%',
            height: 48,
            padding: '0 16px',
            background: 'transparent',
            border: 'none',
            borderBottom: '1px solid rgba(255,255,255,0.08)',
            color: '#fff',
            fontSize: 15,
            outline: 'none',
          }}
        />
        <ul role="listbox" style={{ listStyle: 'none', margin: 0, padding: 4, maxHeight: 320, overflowY: 'auto' }}>
          {hits.map((s, i) => (
            <li key={s.slug}>
              <button
                type="button"
                role="option"
                aria-selected={i === cursor}
                onMouseEnter={() => setCursor(i)}
                onClick={() => go(s.slug)}
                style={{
                  display: 'flex',
                  width: '100%',
                  minHeight: 40,
                  alignItems: 'center',
                  padding: '0 12px',
                  border: 'none',
                  borderRadius: 8,
                  background: i === cursor ? 'rgba(255,255,255,0.08)' : 'transparent',
                  color: i === cursor ? '#fff' : 'rgba(255,255,255,0.75)',
                  fontSize: 14,
                  textAlign: 'left',
                  cursor: 'pointer',
                }}
              >
                {s.label}
              </button>
            </li>
          ))}
          {hits.length === 0 && (
            <li style={{ padding: '12px', fontSize: 13, color: 'rgba(255,255,255,0.5)' }}>Nothing matches “{query}”.</li>
          )}
        </ul>
      </div>
    </div>
  )
}
