'use client'

import { Sidebar } from './sidebar'
import { Topbar } from './topbar'

export function Shell({ children }: { children: React.ReactNode }) {
  return (
    <>
      <Sidebar />
      <div className="pl-64">
        <Topbar />
        <main className="min-h-[calc(100vh-3.5rem)]">{children}</main>
      </div>
    </>
  )
}
