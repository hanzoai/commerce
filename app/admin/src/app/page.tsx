const stats = [
  {
    label: 'Total Revenue',
    value: '$0.00',
    change: '--',
    icon: RevenueIcon,
  },
  {
    label: 'Orders',
    value: '0',
    change: '--',
    icon: OrdersStatIcon,
  },
  {
    label: 'Customers',
    value: '0',
    change: '--',
    icon: CustomersStatIcon,
  },
  {
    label: 'Products',
    value: '0',
    change: '--',
    icon: ProductsStatIcon,
  },
]

function RevenueIcon() {
  return (
    <svg className="h-5 w-5 text-hanzo-red" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 6v12m-3-2.818.879.659c1.171.879 3.07.879 4.242 0 1.172-.879 1.172-2.303 0-3.182C13.536 12.219 12.768 12 12 12c-.725 0-1.45-.22-2.003-.659-1.106-.879-1.106-2.303 0-3.182s2.9-.879 4.006 0l.415.33M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
    </svg>
  )
}

function OrdersStatIcon() {
  return (
    <svg className="h-5 w-5 text-hanzo-red" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 10.5V6a3.75 3.75 0 1 0-7.5 0v4.5m11.356-1.993 1.263 12c.07.665-.45 1.243-1.119 1.243H4.25a1.125 1.125 0 0 1-1.12-1.243l1.264-12A1.125 1.125 0 0 1 5.513 7.5h12.974c.576 0 1.059.435 1.119 1.007ZM8.625 10.5a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm7.5 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z" />
    </svg>
  )
}

function CustomersStatIcon() {
  return (
    <svg className="h-5 w-5 text-hanzo-red" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" d="M18 18.72a9.094 9.094 0 0 0 3.741-.479 3 3 0 0 0-4.682-2.72m.94 3.198.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0 1 12 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 0 1 6 18.719m12 0a5.971 5.971 0 0 0-.941-3.197m0 0A5.995 5.995 0 0 0 12 12.75a5.995 5.995 0 0 0-5.058 2.772m0 0a3 3 0 0 0-4.681 2.72 8.986 8.986 0 0 0 3.74.477m.94-3.197a5.971 5.971 0 0 0-.94 3.197M15 6.75a3 3 0 1 1-6 0 3 3 0 0 1 6 0Zm6 3a2.25 2.25 0 1 1-4.5 0 2.25 2.25 0 0 1 4.5 0Zm-13.5 0a2.25 2.25 0 1 1-4.5 0 2.25 2.25 0 0 1 4.5 0Z" />
    </svg>
  )
}

function ProductsStatIcon() {
  return (
    <svg className="h-5 w-5 text-hanzo-red" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" d="m21 7.5-9-5.25L3 7.5m18 0-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9" />
    </svg>
  )
}

export default function DashboardPage() {
  return (
    <div className="p-8">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-white">Dashboard</h1>
        <p className="mt-1 text-sm text-muted">
          Overview of your commerce operations
        </p>
      </div>

      <div className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {stats.map((stat) => (
          <div key={stat.label} className="card-hover">
            <div className="flex items-center justify-between">
              <p className="text-sm text-muted">{stat.label}</p>
              <stat.icon />
            </div>
            <p className="mt-2 text-3xl font-bold text-white">{stat.value}</p>
            <p className="mt-1 text-xs text-muted">{stat.change}</p>
          </div>
        ))}
      </div>

      <div className="card">
        <div className="flex flex-col items-center justify-center py-16 text-center">
          <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-surface-overlay">
            <svg className="h-8 w-8 text-hanzo-red" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75Z" />
            </svg>
          </div>
          <h2 className="text-xl font-semibold text-white">
            Full Dashboard Coming Soon
          </h2>
          <p className="mt-2 max-w-md text-sm text-muted">
            The complete Hanzo Commerce admin experience is being built. Product management,
            order processing, customer insights, and inventory tracking will be available here.
          </p>
          <div className="mt-6 flex gap-3">
            <a href="https://commerce.hanzo.ai" className="btn-secondary">
              Visit Storefront
            </a>
            <a href="https://docs.hanzo.ai" className="btn-primary">
              Read the Docs
            </a>
          </div>
        </div>
      </div>

      <div className="mt-8 grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div className="card">
          <h3 className="text-sm font-semibold text-white">Recent Orders</h3>
          <p className="mt-4 text-center text-sm text-muted py-8">
            No orders yet
          </p>
        </div>
        <div className="card">
          <h3 className="text-sm font-semibold text-white">Top Products</h3>
          <p className="mt-4 text-center text-sm text-muted py-8">
            No products yet
          </p>
        </div>
      </div>
    </div>
  )
}
