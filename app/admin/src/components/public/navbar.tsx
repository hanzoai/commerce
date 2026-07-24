'use client'

import Link from 'next/link'
import { HanzoMark } from '@/components/hanzo-mark'

export function PublicNavbar() {
  return (
    <nav className="fixed top-0 z-50 w-full border-b border-white/[0.06] bg-[#0a0a0a]/80 backdrop-blur-xl">
      <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-6">
        <Link href="/" className="flex items-center gap-3">
          <HanzoMark className="h-8 w-8 text-white" />
          <span className="text-lg font-semibold text-white">Hanzo Commerce</span>
        </Link>
        <div className="hidden items-center gap-8 text-sm md:flex">
          <a href="#features" className="text-gray-400 transition-colors hover:text-white">Features</a>
          <a href="https://docs.hanzo.ai" className="text-gray-400 transition-colors hover:text-white">Docs</a>
          <a href="https://github.com/hanzoai/commerce" className="text-gray-400 transition-colors hover:text-white">GitHub</a>
          <Link
            href="/login"
            className="rounded-lg border border-white/10 bg-white/5 px-4 py-2 text-white transition-all hover:bg-white/10"
          >
            Sign In
          </Link>
        </div>
      </div>
    </nav>
  )
}
