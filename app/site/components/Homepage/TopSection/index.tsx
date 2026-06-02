const HomepageTopSection = () => {
  return (
    <div className="w-full py-16 px-4 flex flex-col gap-3 justify-center items-center">
      <div className="flex gap-2 items-center text-sm text-gray-500">
        <span className="px-2 py-0.5 rounded border border-gray-200 dark:border-gray-700">
          Hanzo Commerce Documentation
        </span>
        <a
          href="/learn"
          className="text-blue-600 dark:text-blue-400 hover:underline"
        >
          Introduction
        </a>
      </div>
      <div className="flex flex-col gap-3 justify-center items-center max-w-2xl">
        <h1 className="text-4xl md:text-5xl font-bold text-center leading-tight">
          AI-powered commerce infrastructure for modern businesses.
        </h1>
        <p className="text-lg text-gray-600 dark:text-gray-400 text-center">
          Build, customize, and scale your commerce platform with built-in modules for products, orders, payments, and more.
        </p>
        <div className="flex gap-3 mt-4">
          <a
            href="/learn"
            className="px-6 py-2.5 bg-gray-900 dark:bg-white text-white dark:text-gray-900 rounded-lg font-medium text-sm hover:opacity-90 transition"
          >
            Get Started
          </a>
          <a
            href="https://admin.commerce.hanzo.ai"
            className="px-6 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg font-medium text-sm hover:bg-gray-50 dark:hover:bg-gray-800 transition"
          >
            Dashboard
          </a>
          <a
            href="https://commerce.hanzo.ai/store"
            className="px-6 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg font-medium text-sm hover:bg-gray-50 dark:hover:bg-gray-800 transition"
          >
            Storefront
          </a>
        </div>
      </div>
    </div>
  )
}

export default HomepageTopSection
