'use client'

import { useState } from 'react'
import { Sidebar } from './sidebar'
import { Topbar } from './topbar'
import { AiChatDock } from '@/components/ai/ai-chat-dock'

export function Shell({ children }: { children: React.ReactNode }) {
  const [navOpen, setNavOpen] = useState(false)

  return (
    <>
      <Sidebar open={navOpen} onClose={() => setNavOpen(false)} />
      {navOpen && (
        <button
          type="button"
          aria-label="Close navigation"
          onClick={() => setNavOpen(false)}
          className="fixed inset-0 z-40 bg-ui-bg-overlay lg:hidden"
        />
      )}
      <div className="lg:pl-64">
        <Topbar onOpen={() => setNavOpen(true)} />
        <main className="min-h-[calc(100vh-3.5rem)]">{children}</main>
      </div>
      <AiChatDock />
    </>
  )
}
