export default function NotFoundPage() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-[#0a0a0a] px-6">
      <h1 className="mb-4 text-5xl font-bold text-white">404</h1>
      <p className="mb-8 text-gray-400">
        The page you were looking for does not exist.
      </p>
      <div className="flex gap-4">
        <a
          href="/"
          className="rounded-lg bg-brand px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-brand-500"
        >
          Back to Home
        </a>
        <a
          href="https://github.com/hanzoai/commerce/issues"
          className="rounded-lg border border-white/10 bg-white/5 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-white/10"
        >
          Report Issue
        </a>
      </div>
    </div>
  )
}
