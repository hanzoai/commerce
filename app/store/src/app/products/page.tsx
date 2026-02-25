const ALL_PRODUCTS = [
  { id: "1", name: "Hanzo AI Assistant Pro", price: "$49.99", category: "Software" },
  { id: "2", name: "Neural Engine SDK", price: "$199.00", category: "Developer Tools" },
  { id: "3", name: "Commerce API License", price: "$99.00", category: "APIs" },
  { id: "4", name: "Data Pipeline Starter", price: "$29.99", category: "Data" },
  { id: "5", name: "Edge Compute Module", price: "$149.00", category: "Infrastructure" },
  { id: "6", name: "Hanzo Cloud Credits", price: "$500.00", category: "Cloud" },
  { id: "7", name: "ML Model Hosting", price: "$79.00", category: "Cloud" },
  { id: "8", name: "Analytics Dashboard", price: "$39.99", category: "Software" },
  { id: "9", name: "Webhook Gateway", price: "$19.99", category: "APIs" },
]

function ProductCard({
  product,
}: {
  product: (typeof ALL_PRODUCTS)[number]
}) {
  return (
    <div className="card group overflow-hidden">
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
        <button className="mt-3 w-full rounded-lg border border-surface-700 py-2 text-sm font-medium text-surface-200 transition-colors hover:border-primary-400 hover:bg-primary-400/10 hover:text-primary-400">
          Add to Cart
        </button>
      </div>
    </div>
  )
}

export default function ProductsPage() {
  return (
    <div className="mx-auto max-w-7xl px-4 py-12 sm:px-6 lg:px-8">
      {/* Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-3xl font-bold text-white">All Products</h1>
          <p className="mt-1 text-sm text-surface-400">
            {ALL_PRODUCTS.length} products available
          </p>
        </div>
        <div className="flex items-center gap-3">
          <select className="rounded-lg border border-surface-700 bg-surface-900 px-3 py-2 text-sm text-surface-200 focus:border-primary-400 focus:outline-none focus:ring-1 focus:ring-primary-400">
            <option>All Categories</option>
            <option>Software</option>
            <option>Developer Tools</option>
            <option>APIs</option>
            <option>Cloud</option>
            <option>Data</option>
            <option>Infrastructure</option>
          </select>
          <select className="rounded-lg border border-surface-700 bg-surface-900 px-3 py-2 text-sm text-surface-200 focus:border-primary-400 focus:outline-none focus:ring-1 focus:ring-primary-400">
            <option>Sort by: Featured</option>
            <option>Price: Low to High</option>
            <option>Price: High to Low</option>
            <option>Newest</option>
          </select>
        </div>
      </div>

      {/* Product Grid */}
      <div className="mt-8 grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
        {ALL_PRODUCTS.map((product) => (
          <ProductCard key={product.id} product={product} />
        ))}
      </div>
    </div>
  )
}
