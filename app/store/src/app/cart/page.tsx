function EmptyCartIcon() {
  return (
    <svg
      className="h-24 w-24 text-surface-700"
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="8" cy="21" r="1" />
      <circle cx="19" cy="21" r="1" />
      <path d="M2.05 2.05h2l2.66 12.42a2 2 0 0 0 2 1.58h9.78a2 2 0 0 0 1.95-1.57l1.65-7.43H5.12" />
    </svg>
  )
}

export default function CartPage() {
  return (
    <div className="mx-auto max-w-7xl px-4 py-12 sm:px-6 lg:px-8">
      <h1 className="text-3xl font-bold text-white">Shopping Cart</h1>

      <div className="mt-12 flex flex-col items-center justify-center py-16">
        <EmptyCartIcon />
        <h2 className="mt-6 text-xl font-semibold text-white">
          Your cart is empty
        </h2>
        <p className="mt-2 text-sm text-surface-400">
          Looks like you haven&apos;t added any products yet.
        </p>
        <a href="/products" className="btn-primary mt-8">
          Browse Products
        </a>
      </div>

      {/* Order summary placeholder for when items are in cart */}
      <div className="mt-16 hidden">
        <div className="card p-6">
          <h2 className="text-lg font-semibold text-white">Order Summary</h2>
          <div className="mt-4 space-y-3">
            <div className="flex justify-between text-sm">
              <span className="text-surface-400">Subtotal</span>
              <span className="text-white">$0.00</span>
            </div>
            <div className="flex justify-between text-sm">
              <span className="text-surface-400">Shipping</span>
              <span className="text-white">Free</span>
            </div>
            <div className="flex justify-between text-sm">
              <span className="text-surface-400">Tax</span>
              <span className="text-white">$0.00</span>
            </div>
            <div className="border-t border-surface-700 pt-3">
              <div className="flex justify-between text-base font-semibold">
                <span className="text-white">Total</span>
                <span className="text-primary-400">$0.00</span>
              </div>
            </div>
          </div>
          <button className="btn-primary mt-6 w-full">
            Proceed to Checkout
          </button>
        </div>
      </div>
    </div>
  )
}
