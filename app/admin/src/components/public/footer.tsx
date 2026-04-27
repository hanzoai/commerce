export function PublicFooter() {
  return (
    <footer className="border-t border-white/[0.06] px-6 py-12">
      <div className="mx-auto max-w-7xl">
        <div className="grid gap-8 sm:grid-cols-2 lg:grid-cols-4">
          <div>
            <div className="mb-4 flex items-center gap-2">
              <svg viewBox="0 0 24 24" className="h-7 w-7 text-white" xmlns="http://www.w3.org/2000/svg">
                <path d="M3 2 H7 V10 H17 V2 H21 V22 H17 V14 H7 V22 H3 Z" fill="currentColor"/>
              </svg>
              <span className="font-semibold text-white">Hanzo Commerce</span>
            </div>
            <p className="text-sm text-gray-500">AI-powered commerce infrastructure for modern businesses.</p>
          </div>

          <div>
            <h4 className="mb-3 text-sm font-semibold text-white">Product</h4>
            <ul className="space-y-2 text-sm">
              <li><a href="https://docs.hanzo.ai" className="text-gray-500 transition-colors hover:text-gray-300">Documentation</a></li>
              <li><a href="https://docs.hanzo.ai/api" className="text-gray-500 transition-colors hover:text-gray-300">API Reference</a></li>
              <li><a href="/login" className="text-gray-500 transition-colors hover:text-gray-300">Admin Dashboard</a></li>
            </ul>
          </div>

          <div>
            <h4 className="mb-3 text-sm font-semibold text-white">Company</h4>
            <ul className="space-y-2 text-sm">
              <li><a href="https://hanzo.ai" className="text-gray-500 transition-colors hover:text-gray-300">Hanzo AI</a></li>
              <li><a href="https://hanzo.ai/blog" className="text-gray-500 transition-colors hover:text-gray-300">Blog</a></li>
              <li><a href="https://hanzo.ai/careers" className="text-gray-500 transition-colors hover:text-gray-300">Careers</a></li>
            </ul>
          </div>

          <div>
            <h4 className="mb-3 text-sm font-semibold text-white">Community</h4>
            <ul className="space-y-2 text-sm">
              <li><a href="https://github.com/hanzoai/commerce" className="text-gray-500 transition-colors hover:text-gray-300">GitHub</a></li>
              <li><a href="https://discord.gg/hanzo" className="text-gray-500 transition-colors hover:text-gray-300">Discord</a></li>
              <li><a href="https://x.com/hanzoai" className="text-gray-500 transition-colors hover:text-gray-300">X / Twitter</a></li>
            </ul>
          </div>
        </div>

        <div className="mt-12 flex flex-col items-center justify-between gap-4 border-t border-white/[0.06] pt-8 sm:flex-row">
          <p className="text-sm text-gray-600">&copy; {new Date().getFullYear()} Hanzo Industries, Inc. All rights reserved.</p>
          <div className="flex gap-6 text-sm">
            <a href="https://hanzo.ai/privacy" className="text-gray-600 transition-colors hover:text-gray-400">Privacy</a>
            <a href="https://hanzo.ai/terms" className="text-gray-600 transition-colors hover:text-gray-400">Terms</a>
          </div>
        </div>
      </div>
    </footer>
  )
}
