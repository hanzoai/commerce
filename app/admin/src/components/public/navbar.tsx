'use client'

import Link from 'next/link'

export function PublicNavbar() {
  return (
    <nav className="fixed top-0 z-50 w-full border-b border-white/[0.06] bg-[#0a0a0a]/80 backdrop-blur-xl">
      <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-6">
        <Link href="/" className="flex items-center gap-3">
          <svg viewBox="0 0 67 67" className="h-8 w-8 text-white" xmlns="http://www.w3.org/2000/svg">
            <path d="M22.21 67V44.6369H0V67H22.21Z" fill="currentColor"/>
            <path d="M0 44.6369L22.21 46.8285V44.6369H0Z" fill="currentColor" opacity="0.7"/>
            <path d="M66.7038 22.3184H22.2534L0.0878906 44.6367H44.4634L66.7038 22.3184Z" fill="currentColor"/>
            <path d="M22.21 0H0V22.3184H22.21V0Z" fill="currentColor"/>
            <path d="M66.7198 0H44.5098V22.3184H66.7198V0Z" fill="currentColor"/>
            <path d="M66.6753 22.3185L44.5098 20.0822V22.3185H66.6753Z" fill="currentColor" opacity="0.7"/>
            <path d="M66.7198 67V44.6369H44.5098V67H66.7198Z" fill="currentColor"/>
          </svg>
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
