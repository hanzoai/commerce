"use client"

const FEATURED_PRODUCTS = [
  {
    id: "1",
    name: "Hanzo AI Assistant Pro",
    price: "$49.99",
    category: "Software",
    image: null,
  },
  {
    id: "2",
    name: "Neural Engine SDK",
    price: "$199.00",
    category: "Developer Tools",
    image: null,
  },
  {
    id: "3",
    name: "Commerce API License",
    price: "$99.00",
    category: "APIs",
    image: null,
  },
  {
    id: "4",
    name: "Data Pipeline Starter",
    price: "$29.99",
    category: "Data",
    image: null,
  },
  {
    id: "5",
    name: "Edge Compute Module",
    price: "$149.00",
    category: "Infrastructure",
    image: null,
  },
  {
    id: "6",
    name: "Hanzo Cloud Credits",
    price: "$500.00",
    category: "Cloud",
    image: null,
  },
]

const CATEGORIES = [
  { name: "Software", count: 24, icon: "M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" },
  { name: "Developer Tools", count: 18, icon: "M16 18l6-6-6-6M8 6l-6 6 6 6" },
  { name: "APIs", count: 12, icon: "M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2zM4 8h16M8 4v16" },
  { name: "Cloud", count: 31, icon: "M18 10h-1.26A8 8 0 1 0 9 20h9a5 5 0 0 0 0-10z" },
]

function ProductCard({ product }: { product: typeof FEATURED_PRODUCTS[number] }) {
  return (
    <a href={`/products`} className="card group block overflow-hidden">
      <div className="relative aspect-square bg-surface-800">
        <div className="absolute inset-0 flex items-center justify-center">
          <svg
            className="h-16 w-16 text-surface-600 transition-colors group-hover:text-primary-400/60"
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <rect width="18" height="18" x="3" y="3" rx="2" ry="2" />
            <circle cx="9" cy="9" r="2" />
            <path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21" />
          </svg>
        </div>
        <div className="absolute left-3 top-3">
          <span className="rounded-full bg-surface-950/70 px-2.5 py-1 text-xs font-medium text-surface-300 backdrop-blur-sm">
            {product.category}
          </span>
        </div>
      </div>
      <div className="p-4">
        <h3 className="text-sm font-medium text-white group-hover:text-primary-400 transition-colors">
          {product.name}
        </h3>
        <p className="mt-1 text-lg font-semibold text-primary-400">
          {product.price}
        </p>
      </div>
    </a>
  )
}

function CategoryCard({ category }: { category: typeof CATEGORIES[number] }) {
  return (
    <a
      href="/products"
      className="card group flex items-center gap-4 p-5"
    >
      <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-lg bg-primary-400/10 text-primary-400 transition-colors group-hover:bg-primary-400/20">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="24"
          height="24"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <path d={category.icon} />
        </svg>
      </div>
      <div>
        <h3 className="text-sm font-semibold text-white">
          {category.name}
        </h3>
        <p className="text-xs text-surface-400">
          {category.count} products
        </p>
      </div>
    </a>
  )
}

export default function HomePage() {
  return (
    <div>
      {/* Hero */}
      <section className="relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-br from-primary-400/10 via-surface-950 to-surface-950" />
        <div className="absolute inset-0">
          <div className="absolute -left-1/4 top-0 h-[500px] w-[500px] rounded-full bg-primary-400/5 blur-3xl" />
          <div className="absolute -right-1/4 bottom-0 h-[500px] w-[500px] rounded-full bg-primary-400/5 blur-3xl" />
        </div>
        <div className="relative mx-auto max-w-7xl px-4 py-24 sm:px-6 sm:py-32 lg:px-8 lg:py-40">
          <div className="max-w-2xl">
            <h1 className="text-4xl font-bold tracking-tight text-white sm:text-5xl lg:text-6xl">
              Hanzo Commerce
              <span className="block text-primary-400">Store</span>
            </h1>
            <p className="mt-6 text-lg leading-8 text-surface-300">
              Discover premium AI-powered tools, APIs, and cloud infrastructure.
              Everything you need to build, deploy, and scale intelligent applications.
            </p>
            <div className="mt-10 flex flex-wrap gap-4">
              <a href="/products" className="btn-primary">
                Browse Products
              </a>
              <a href="/products" className="btn-secondary">
                View Collections
              </a>
            </div>
          </div>
        </div>
      </section>

      {/* Featured Products */}
      <section className="mx-auto max-w-7xl px-4 py-16 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-2xl font-bold text-white">
              Featured Products
            </h2>
            <p className="mt-1 text-sm text-surface-400">
              Our most popular tools and services
            </p>
          </div>
          <a
            href="/products"
            className="text-sm font-medium text-primary-400 hover:text-primary-300 transition-colors"
          >
            View all &rarr;
          </a>
        </div>
        <div className="mt-8 grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {FEATURED_PRODUCTS.map((product) => (
            <ProductCard key={product.id} product={product} />
          ))}
        </div>
      </section>

      {/* Categories */}
      <section className="mx-auto max-w-7xl px-4 py-16 sm:px-6 lg:px-8">
        <div>
          <h2 className="text-2xl font-bold text-white">
            Shop by Category
          </h2>
          <p className="mt-1 text-sm text-surface-400">
            Browse products by type
          </p>
        </div>
        <div className="mt-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {CATEGORIES.map((category) => (
            <CategoryCard key={category.name} category={category} />
          ))}
        </div>
      </section>

      {/* Newsletter */}
      <section className="border-t border-surface-800 bg-surface-900/30">
        <div className="mx-auto max-w-7xl px-4 py-16 sm:px-6 lg:px-8">
          <div className="mx-auto max-w-xl text-center">
            <h2 className="text-2xl font-bold text-white">
              Stay in the Loop
            </h2>
            <p className="mt-2 text-sm text-surface-400">
              Get notified about new products, exclusive deals, and platform updates.
            </p>
            <form className="mt-6 flex gap-3" onSubmit={(e) => e.preventDefault()}>
              <input
                type="email"
                placeholder="you@example.com"
                className="flex-1 rounded-lg border border-surface-700 bg-surface-900 px-4 py-3 text-sm text-surface-100 placeholder-surface-500 focus:border-primary-400 focus:outline-none focus:ring-1 focus:ring-primary-400"
              />
              <button type="submit" className="btn-primary whitespace-nowrap">
                Subscribe
              </button>
            </form>
            <p className="mt-3 text-xs text-surface-500">
              No spam. Unsubscribe anytime.
            </p>
          </div>
        </div>
      </section>
    </div>
  )
}
