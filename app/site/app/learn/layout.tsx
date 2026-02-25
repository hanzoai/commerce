import Footer from "../../components/Footer"

export default function LearnLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <body className="font-sans text-base text-gray-900 dark:text-gray-100">
      <nav className="border-b border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900 px-4 py-3 flex items-center justify-between sticky top-0 z-50">
        <a href="/" className="flex items-center gap-2">
          <span className="text-xl font-bold">Hanzo Commerce</span>
        </a>
        <div className="flex items-center gap-4 text-sm">
          <a href="/learn" className="hover:text-blue-600 dark:hover:text-blue-400">Docs</a>
          <a href="https://admin.commerce.hanzo.ai" className="hover:text-blue-600 dark:hover:text-blue-400">Dashboard</a>
          <a href="https://github.com/hanzoai/commerce" className="hover:text-blue-600 dark:hover:text-blue-400">GitHub</a>
        </div>
      </nav>
      <main className="max-w-4xl mx-auto px-4 py-8 prose dark:prose-invert">
        {children}
      </main>
      <Footer />
    </body>
  )
}
