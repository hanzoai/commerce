import Footer from "../components/Footer"

const NotFoundPage = () => {
  return (
    <body className="font-sans text-base text-gray-900 dark:text-gray-100">
      <nav className="border-b border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900 px-4 py-3 flex items-center justify-between">
        <a href="/" className="flex items-center gap-2">
          <span className="text-xl font-bold">Hanzo Commerce</span>
        </a>
      </nav>
      <main className="max-w-4xl mx-auto px-4 py-16 text-center">
        <h1 className="text-4xl font-bold mb-4">Page Not Found</h1>
        <p className="text-gray-500 mb-8">
          The page you were looking for is not available.
        </p>
        <div className="flex gap-4 justify-center">
          <a
            href="/learn"
            className="px-4 py-2 bg-gray-900 dark:bg-white text-white dark:text-gray-900 rounded-lg text-sm font-medium"
          >
            Get Started Docs
          </a>
          <a
            href="https://github.com/hanzoai/commerce/issues"
            className="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg text-sm font-medium"
          >
            Report Issue
          </a>
        </div>
      </main>
      <Footer />
    </body>
  )
}

export default NotFoundPage
