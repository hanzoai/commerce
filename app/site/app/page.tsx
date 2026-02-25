import Providers from "../providers"

const FEATURES = [
  {
    title: "Headless API",
    description:
      "RESTful and GraphQL APIs designed for any frontend. Build custom storefronts with complete flexibility and zero vendor lock-in.",
    icon: (
      <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M17.25 6.75L22.5 12l-5.25 5.25m-10.5 0L1.5 12l5.25-5.25m7.5-3l-4.5 16.5" />
      </svg>
    ),
  },
  {
    title: "Multi-Currency",
    description:
      "Native support for 100+ currencies with real-time exchange rates. Localized pricing and tax calculation built in.",
    icon: (
      <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M12 6v12m-3-2.818l.879.659c1.171.879 3.07.879 4.242 0 1.172-.879 1.172-2.303 0-3.182C13.536 12.219 12.768 12 12 12c-.725 0-1.45-.22-2.003-.659-1.106-.879-1.106-2.303 0-3.182s2.9-.879 4.006 0l.415.33M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
    ),
  },
  {
    title: "AI Pricing",
    description:
      "Machine learning models that optimize pricing in real time. Maximize revenue with dynamic pricing strategies and demand forecasting.",
    icon: (
      <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.455 2.456L21.75 6l-1.036.259a3.375 3.375 0 00-2.455 2.456z" />
      </svg>
    ),
  },
  {
    title: "Real-time Analytics",
    description:
      "Live dashboards with conversion funnels, cohort analysis, and revenue attribution. Make data-driven decisions instantly.",
    icon: (
      <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z" />
      </svg>
    ),
  },
  {
    title: "Storefront Builder",
    description:
      "Pre-built, customizable storefront templates powered by Next.js. Launch a production-ready store in minutes, not months.",
    icon: (
      <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M13.5 21v-7.5a.75.75 0 01.75-.75h3a.75.75 0 01.75.75V21m-4.5 0H2.36m11.14 0H18m0 0h3.64m-1.39 0V9.349m-16.5 11.65V9.35m0 0a3.001 3.001 0 003.75-.615A2.993 2.993 0 009.75 9.75c.896 0 1.7-.393 2.25-1.016a2.993 2.993 0 002.25 1.016c.896 0 1.7-.393 2.25-1.016A3.001 3.001 0 0021 9.349m-18 0a2.999 2.999 0 01.57-1.771L5.07 5.534A2.25 2.25 0 016.894 4.5h10.212a2.25 2.25 0 011.824 1.034l1.5 2.044A3 3 0 0121 9.35" />
      </svg>
    ),
  },
  {
    title: "Admin Dashboard",
    description:
      "Full-featured admin panel for order management, inventory, customers, and settings. Extensible with custom widgets and views.",
    icon: (
      <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M10.5 6h9.75M10.5 6a1.5 1.5 0 11-3 0m3 0a1.5 1.5 0 10-3 0M3.75 6H7.5m3 12h9.75m-9.75 0a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m-3.75 0H7.5m9-6h3.75m-3.75 0a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m-9.75 0h9.75" />
      </svg>
    ),
  },
]

const CODE_EXAMPLE = `// Create a product with the Hanzo Commerce API
const product = await hanzo.products.create({
  title: "Premium Wireless Headphones",
  handle: "premium-wireless-headphones",
  status: "published",
  variants: [
    {
      title: "Matte Black",
      prices: [
        { amount: 29900, currency_code: "usd" },
        { amount: 27900, currency_code: "eur" },
      ],
      manage_inventory: true,
      inventory_quantity: 500,
    },
  ],
})

// AI-optimized pricing suggestion
const pricing = await hanzo.ai.pricing.optimize({
  product_id: product.id,
  strategy: "maximize_revenue",
  constraints: { min_margin: 0.35 },
})`

export default function HomePage() {
  return (
    <Providers>
      <div className="min-h-screen bg-[#0a0a0a]">
        {/* Navigation */}
        <nav className="fixed top-0 z-50 w-full border-b border-white/[0.06] bg-[#0a0a0a]/80 backdrop-blur-xl">
          <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-6">
            <a href="/" className="flex items-center gap-3">
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-brand">
                <span className="text-sm font-bold text-white">H</span>
              </div>
              <span className="text-lg font-semibold text-white">
                Hanzo Commerce
              </span>
            </a>
            <div className="hidden items-center gap-8 text-sm md:flex">
              <a
                href="https://docs.commerce.hanzo.ai"
                className="text-gray-400 transition-colors hover:text-white"
              >
                Docs
              </a>
              <a
                href="https://admin.commerce.hanzo.ai"
                className="text-gray-400 transition-colors hover:text-white"
              >
                Dashboard
              </a>
              <a
                href="https://github.com/hanzoai/commerce"
                className="text-gray-400 transition-colors hover:text-white"
              >
                GitHub
              </a>
              <a
                href="https://hanzo.id/login/oauth/authorize?client_id=hanzo-app-client-id&response_type=code&redirect_uri=https://commerce.hanzo.ai/auth/callback&scope=openid+profile+email&state=https://admin.commerce.hanzo.ai"
                className="rounded-lg border border-white/10 bg-white/5 px-4 py-2 text-white transition-all hover:bg-white/10"
              >
                Sign In
              </a>
            </div>
          </div>
        </nav>

        {/* Hero Section */}
        <section className="relative flex min-h-screen flex-col items-center justify-center overflow-hidden px-6 pt-16">
          {/* Background gradient effects */}
          <div className="pointer-events-none absolute inset-0">
            <div className="absolute left-1/2 top-0 h-[600px] w-[800px] -translate-x-1/2 rounded-full bg-brand/[0.04] blur-[120px]" />
            <div className="absolute bottom-0 left-1/4 h-[400px] w-[600px] rounded-full bg-brand/[0.02] blur-[100px]" />
          </div>

          <div className="relative z-10 mx-auto max-w-4xl text-center">
            <div className="mb-6 inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/5 px-4 py-1.5 text-sm text-gray-400">
              <span className="h-1.5 w-1.5 rounded-full bg-brand animate-pulse" />
              Now in Public Beta
            </div>

            <h1 className="mb-6 text-5xl font-bold leading-tight tracking-tight text-white sm:text-6xl lg:text-7xl">
              <span className="bg-gradient-to-r from-white via-white to-gray-500 bg-clip-text text-transparent">
                AI-Powered
              </span>
              <br />
              <span className="bg-gradient-to-r from-brand-400 via-brand to-orange-400 bg-clip-text text-transparent">
                Commerce Platform
              </span>
            </h1>

            <p className="mx-auto mb-10 max-w-2xl text-lg leading-relaxed text-gray-400 sm:text-xl">
              Build, launch, and scale your commerce business with intelligent
              APIs. Headless architecture, multi-currency support, AI-driven
              pricing, and real-time analytics -- all from a single platform.
            </p>

            <div className="flex flex-col items-center gap-4 sm:flex-row sm:justify-center">
              <a
                href="https://docs.commerce.hanzo.ai"
                className="group inline-flex items-center gap-2 rounded-xl bg-brand px-8 py-3.5 text-sm font-semibold text-white shadow-lg shadow-brand/20 transition-all hover:bg-brand-500 hover:shadow-brand/30"
              >
                Get Started
                <svg
                  className="h-4 w-4 transition-transform group-hover:translate-x-0.5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M13.5 4.5L21 12m0 0l-7.5 7.5M21 12H3"
                  />
                </svg>
              </a>
              <a
                href="https://github.com/hanzoai/commerce"
                className="inline-flex items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-8 py-3.5 text-sm font-semibold text-white transition-all hover:bg-white/10"
              >
                <svg className="h-5 w-5" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
                </svg>
                View on GitHub
              </a>
            </div>
          </div>

          {/* Scroll indicator */}
          <div className="absolute bottom-8 left-1/2 -translate-x-1/2">
            <div className="flex h-8 w-5 items-start justify-center rounded-full border border-white/20 p-1">
              <div className="h-2 w-1 animate-bounce rounded-full bg-white/40" />
            </div>
          </div>
        </section>

        {/* Features Grid */}
        <section className="relative px-6 py-32">
          <div className="mx-auto max-w-7xl">
            <div className="mb-16 text-center">
              <h2 className="mb-4 text-3xl font-bold text-white sm:text-4xl">
                Everything you need to sell online
              </h2>
              <p className="mx-auto max-w-2xl text-lg text-gray-400">
                A modular, extensible commerce engine built for developers who
                demand performance, flexibility, and intelligence.
              </p>
            </div>

            <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
              {FEATURES.map((feature) => (
                <div
                  key={feature.title}
                  className="group rounded-2xl border border-white/[0.06] bg-white/[0.02] p-8 transition-all hover:border-white/[0.12] hover:bg-white/[0.04]"
                >
                  <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-brand/10 text-brand transition-colors group-hover:bg-brand/20">
                    {feature.icon}
                  </div>
                  <h3 className="mb-2 text-lg font-semibold text-white">
                    {feature.title}
                  </h3>
                  <p className="text-sm leading-relaxed text-gray-400">
                    {feature.description}
                  </p>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* Code Preview Section */}
        <section className="relative px-6 py-32">
          <div className="pointer-events-none absolute inset-0">
            <div className="absolute right-1/4 top-1/2 h-[400px] w-[600px] -translate-y-1/2 rounded-full bg-brand/[0.03] blur-[100px]" />
          </div>

          <div className="relative mx-auto max-w-7xl">
            <div className="grid items-center gap-16 lg:grid-cols-2">
              <div>
                <h2 className="mb-4 text-3xl font-bold text-white sm:text-4xl">
                  Developer-first API
                </h2>
                <p className="mb-8 text-lg leading-relaxed text-gray-400">
                  Clean, predictable APIs that do exactly what you expect. Create
                  products, manage inventory, process payments, and leverage AI
                  pricing -- all with a few lines of code.
                </p>
                <div className="flex flex-col gap-4 sm:flex-row">
                  <a
                    href="https://docs.commerce.hanzo.ai"
                    className="inline-flex items-center gap-2 text-sm font-medium text-brand transition-colors hover:text-brand-300"
                  >
                    Read the documentation
                    <svg
                      className="h-4 w-4"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      strokeWidth={2}
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        d="M13.5 4.5L21 12m0 0l-7.5 7.5M21 12H3"
                      />
                    </svg>
                  </a>
                  <a
                    href="https://docs.commerce.hanzo.ai/api"
                    className="inline-flex items-center gap-2 text-sm font-medium text-gray-400 transition-colors hover:text-white"
                  >
                    API Reference
                    <svg
                      className="h-4 w-4"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      strokeWidth={2}
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        d="M13.5 4.5L21 12m0 0l-7.5 7.5M21 12H3"
                      />
                    </svg>
                  </a>
                </div>
              </div>

              <div className="overflow-hidden rounded-2xl border border-white/[0.06] bg-[#111111]">
                <div className="flex items-center gap-2 border-b border-white/[0.06] px-4 py-3">
                  <div className="h-3 w-3 rounded-full bg-white/10" />
                  <div className="h-3 w-3 rounded-full bg-white/10" />
                  <div className="h-3 w-3 rounded-full bg-white/10" />
                  <span className="ml-2 text-xs text-gray-500">
                    commerce.ts
                  </span>
                </div>
                <pre className="overflow-x-auto p-6 text-sm leading-relaxed">
                  <code className="font-mono text-gray-300">
                    {CODE_EXAMPLE}
                  </code>
                </pre>
              </div>
            </div>
          </div>
        </section>

        {/* CTA Section */}
        <section className="relative px-6 py-32">
          <div className="mx-auto max-w-4xl text-center">
            <h2 className="mb-4 text-3xl font-bold text-white sm:text-4xl">
              Ready to build?
            </h2>
            <p className="mx-auto mb-10 max-w-xl text-lg text-gray-400">
              Start building your commerce platform today. Free during beta --
              no credit card required.
            </p>

            <div className="flex flex-col items-center gap-4 sm:flex-row sm:justify-center">
              <a
                href="https://docs.commerce.hanzo.ai"
                className="group inline-flex items-center gap-2 rounded-xl bg-brand px-8 py-3.5 text-sm font-semibold text-white shadow-lg shadow-brand/20 transition-all hover:bg-brand-500 hover:shadow-brand/30"
              >
                Get Started
                <svg
                  className="h-4 w-4 transition-transform group-hover:translate-x-0.5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M13.5 4.5L21 12m0 0l-7.5 7.5M21 12H3"
                  />
                </svg>
              </a>
              <a
                href="https://hanzo.id/login/oauth/authorize?client_id=hanzo-app-client-id&response_type=code&redirect_uri=https://commerce.hanzo.ai/auth/callback&scope=openid+profile+email&state=https://admin.commerce.hanzo.ai"
                className="inline-flex items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-8 py-3.5 text-sm font-semibold text-white transition-all hover:bg-white/10"
              >
                Sign In
              </a>
            </div>
          </div>
        </section>

        {/* Footer */}
        <footer className="border-t border-white/[0.06] px-6 py-12">
          <div className="mx-auto max-w-7xl">
            <div className="grid gap-8 sm:grid-cols-2 lg:grid-cols-4">
              <div>
                <div className="mb-4 flex items-center gap-2">
                  <div className="flex h-7 w-7 items-center justify-center rounded-md bg-brand">
                    <span className="text-xs font-bold text-white">H</span>
                  </div>
                  <span className="font-semibold text-white">
                    Hanzo Commerce
                  </span>
                </div>
                <p className="text-sm text-gray-500">
                  AI-powered commerce infrastructure for modern businesses.
                </p>
              </div>

              <div>
                <h4 className="mb-3 text-sm font-semibold text-white">
                  Product
                </h4>
                <ul className="space-y-2 text-sm">
                  <li>
                    <a
                      href="https://docs.commerce.hanzo.ai"
                      className="text-gray-500 transition-colors hover:text-gray-300"
                    >
                      Documentation
                    </a>
                  </li>
                  <li>
                    <a
                      href="https://docs.commerce.hanzo.ai/api"
                      className="text-gray-500 transition-colors hover:text-gray-300"
                    >
                      API Reference
                    </a>
                  </li>
                  <li>
                    <a
                      href="https://admin.commerce.hanzo.ai"
                      className="text-gray-500 transition-colors hover:text-gray-300"
                    >
                      Admin Dashboard
                    </a>
                  </li>
                  <li>
                    <a
                      href="https://store.commerce.hanzo.ai"
                      className="text-gray-500 transition-colors hover:text-gray-300"
                    >
                      Storefront
                    </a>
                  </li>
                </ul>
              </div>

              <div>
                <h4 className="mb-3 text-sm font-semibold text-white">
                  Company
                </h4>
                <ul className="space-y-2 text-sm">
                  <li>
                    <a
                      href="https://hanzo.ai"
                      className="text-gray-500 transition-colors hover:text-gray-300"
                    >
                      Hanzo AI
                    </a>
                  </li>
                  <li>
                    <a
                      href="https://hanzo.ai/blog"
                      className="text-gray-500 transition-colors hover:text-gray-300"
                    >
                      Blog
                    </a>
                  </li>
                  <li>
                    <a
                      href="https://hanzo.ai/careers"
                      className="text-gray-500 transition-colors hover:text-gray-300"
                    >
                      Careers
                    </a>
                  </li>
                </ul>
              </div>

              <div>
                <h4 className="mb-3 text-sm font-semibold text-white">
                  Community
                </h4>
                <ul className="space-y-2 text-sm">
                  <li>
                    <a
                      href="https://github.com/hanzoai/commerce"
                      className="text-gray-500 transition-colors hover:text-gray-300"
                    >
                      GitHub
                    </a>
                  </li>
                  <li>
                    <a
                      href="https://discord.gg/hanzo"
                      className="text-gray-500 transition-colors hover:text-gray-300"
                    >
                      Discord
                    </a>
                  </li>
                  <li>
                    <a
                      href="https://x.com/hanaboroshi"
                      className="text-gray-500 transition-colors hover:text-gray-300"
                    >
                      X / Twitter
                    </a>
                  </li>
                </ul>
              </div>
            </div>

            <div className="mt-12 flex flex-col items-center justify-between gap-4 border-t border-white/[0.06] pt-8 sm:flex-row">
              <p className="text-sm text-gray-600">
                &copy; {new Date().getFullYear()} Hanzo Industries, Inc. All
                rights reserved.
              </p>
              <div className="flex gap-6 text-sm">
                <a
                  href="https://hanzo.ai/privacy"
                  className="text-gray-600 transition-colors hover:text-gray-400"
                >
                  Privacy
                </a>
                <a
                  href="https://hanzo.ai/terms"
                  className="text-gray-600 transition-colors hover:text-gray-400"
                >
                  Terms
                </a>
              </div>
            </div>
          </div>
        </footer>
      </div>
    </Providers>
  )
}
