'use client'

/**
 * Who is signed in, and how they leave.
 *
 * The top bar rendered `currentOrgId` as a bare string and offered no way to
 * change it, no name, and no sign-out — so the one control a merchant needs on
 * every screen (which store am I editing) was a label, and the way out was
 * wherever they last saw it.
 *
 * The ORGANIZATION half is not built here: `@hanzo/iam/react` publishes
 * `OrgProjectSwitcher` and `useOrganizations` feeds it directly. Every surface
 * on the estate should be switching orgs through that one component, and a
 * second one written locally is exactly how two products come to disagree about
 * what an org is.
 */

import { useEffect, useRef, useState } from 'react'
import { useIam } from '@hanzo/iam/react'

export function Account() {
  const { user, logout } = useIam()
  const [open, setOpen] = useState(false)
  const box = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const away = (e: MouseEvent) => {
      if (box.current && !box.current.contains(e.target as Node)) setOpen(false)
    }
    const key = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', away)
    document.addEventListener('keydown', key)
    return () => {
      document.removeEventListener('mousedown', away)
      document.removeEventListener('keydown', key)
    }
  }, [open])

  const name = user?.name || user?.email || ''
  if (!name) return null

  return (
    <div ref={box} style={{ position: 'relative' }}>
      <button
        type="button"
        onClick={() => setOpen(!open)}
        aria-expanded={open}
        aria-haspopup="menu"
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: 8,
          minHeight: 32,
          padding: '0 6px',
          background: 'transparent',
          border: 'none',
          borderRadius: 8,
          color: 'rgba(255,255,255,0.75)',
          fontSize: 13,
          cursor: 'pointer',
        }}
      >
        <span
          aria-hidden
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: 24,
            height: 24,
            borderRadius: '50%',
            background: 'rgba(255,255,255,0.1)',
            color: '#fff',
            fontSize: 11,
            textTransform: 'uppercase',
          }}
        >
          {name.slice(0, 1)}
        </span>
        <span style={{ maxWidth: 160, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{name}</span>
      </button>

      {open && (
        <div
          role="menu"
          style={{
            position: 'absolute',
            right: 0,
            top: '100%',
            marginTop: 8,
            minWidth: 220,
            background: '#0a0a0a',
            border: '1px solid rgba(255,255,255,0.12)',
            borderRadius: 10,
            overflow: 'hidden',
            boxShadow: '0 16px 48px rgba(0,0,0,0.6)',
            zIndex: 50,
          }}
        >
          <div style={{ padding: '10px 14px', borderBottom: '1px solid rgba(255,255,255,0.08)' }}>
            <div style={{ fontSize: 13, color: '#fff' }}>{user?.name || user?.email}</div>
            {user?.name && user?.email ? (
              <div style={{ fontSize: 12, color: 'rgba(255,255,255,0.55)' }}>{user.email}</div>
            ) : null}
          </div>
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              setOpen(false)
              logout()
            }}
            style={{
              display: 'flex',
              width: '100%',
              minHeight: 40,
              alignItems: 'center',
              padding: '0 14px',
              background: 'transparent',
              border: 'none',
              color: 'rgba(255,255,255,0.75)',
              fontSize: 13,
              textAlign: 'left',
              cursor: 'pointer',
            }}
          >
            Sign out
          </button>
        </div>
      )}
    </div>
  )
}
