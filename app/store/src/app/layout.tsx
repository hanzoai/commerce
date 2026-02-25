import type { Metadata } from "next"
import "./globals.css"

export const metadata: Metadata = {
  title: "Hanzo Commerce Store",
  description: "The official Hanzo Commerce storefront",
}

function SearchIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="11" cy="11" r="8" />
      <path d="m21 21-4.3-4.3" />
    </svg>
  )
}

function CartIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="8" cy="21" r="1" />
      <circle cx="19" cy="21" r="1" />
      <path d="M2.05 2.05h2l2.66 12.42a2 2 0 0 0 2 1.58h9.78a2 2 0 0 0 1.95-1.57l1.65-7.43H5.12" />
    </svg>
  )
}

function Navbar() {
  return (
    <header className="sticky top-0 z-50 border-b border-surface-800 bg-surface-950/80 backdrop-blur-xl">
      <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 sm:px-6 lg:px-8">
        <div className="flex items-center gap-8">
          <a href="/" className="flex items-center gap-2">
            <span className="text-xl font-bold tracking-tight text-white">
              HANZO
            </span>
            <span className="text-xs font-medium uppercase tracking-widest text-primary-400">
              Store
            </span>
          </a>
          <nav className="hidden md:flex items-center gap-6">
            <a
              href="/products"
              className="text-sm text-surface-300 transition-colors hover:text-white"
            >
              Products
            </a>
            <a
              href="/products"
              className="text-sm text-surface-300 transition-colors hover:text-white"
            >
              Collections
            </a>
            <a
              href="/products"
              className="text-sm text-surface-300 transition-colors hover:text-white"
            >
              New Arrivals
            </a>
          </nav>
        </div>

        <div className="flex items-center gap-4">
          <div className="hidden sm:flex items-center">
            <div className="relative">
              <input
                type="text"
                placeholder="Search products..."
                className="h-9 w-64 rounded-lg border border-surface-700 bg-surface-900 pl-9 pr-4 text-sm text-surface-100 placeholder-surface-500 focus:border-primary-400 focus:outline-none focus:ring-1 focus:ring-primary-400"
              />
              <div className="absolute left-2.5 top-1/2 -translate-y-1/2 text-surface-500">
                <SearchIcon />
              </div>
            </div>
          </div>
          <a
            href="/cart"
            className="relative flex h-9 w-9 items-center justify-center rounded-lg text-surface-300 transition-colors hover:bg-surface-800 hover:text-white"
          >
            <CartIcon />
            <span className="absolute -right-1 -top-1 flex h-4 w-4 items-center justify-center rounded-full bg-primary-400 text-[10px] font-bold text-white">
              0
            </span>
          </a>
          <a
            href="#"
            className="hidden sm:inline-flex h-9 items-center rounded-lg border border-surface-700 px-4 text-sm font-medium text-surface-200 transition-colors hover:bg-surface-800 hover:text-white"
          >
            Sign In
          </a>
        </div>
      </div>
    </header>
  )
}

function Footer() {
  return (
    <footer className="border-t border-surface-800 bg-surface-950">
      <div className="mx-auto max-w-7xl px-4 py-12 sm:px-6 lg:px-8">
        <div className="grid grid-cols-1 gap-8 sm:grid-cols-2 lg:grid-cols-4">
          <div>
            <h3 className="text-sm font-semibold uppercase tracking-wider text-white">
              Shop
            </h3>
            <ul className="mt-4 space-y-2">
              <li>
                <a href="/products" className="text-sm text-surface-400 hover:text-white">
                  All Products
                </a>
              </li>
              <li>
                <a href="/products" className="text-sm text-surface-400 hover:text-white">
                  New Arrivals
                </a>
              </li>
              <li>
                <a href="/products" className="text-sm text-surface-400 hover:text-white">
                  Best Sellers
                </a>
              </li>
            </ul>
          </div>
          <div>
            <h3 className="text-sm font-semibold uppercase tracking-wider text-white">
              Support
            </h3>
            <ul className="mt-4 space-y-2">
              <li>
                <a href="#" className="text-sm text-surface-400 hover:text-white">
                  Contact Us
                </a>
              </li>
              <li>
                <a href="#" className="text-sm text-surface-400 hover:text-white">
                  Shipping Info
                </a>
              </li>
              <li>
                <a href="#" className="text-sm text-surface-400 hover:text-white">
                  Returns
                </a>
              </li>
            </ul>
          </div>
          <div>
            <h3 className="text-sm font-semibold uppercase tracking-wider text-white">
              Company
            </h3>
            <ul className="mt-4 space-y-2">
              <li>
                <a href="https://hanzo.ai" className="text-sm text-surface-400 hover:text-white">
                  About Hanzo
                </a>
              </li>
              <li>
                <a href="#" className="text-sm text-surface-400 hover:text-white">
                  Careers
                </a>
              </li>
              <li>
                <a href="#" className="text-sm text-surface-400 hover:text-white">
                  Blog
                </a>
              </li>
            </ul>
          </div>
          <div>
            <h3 className="text-sm font-semibold uppercase tracking-wider text-white">
              Legal
            </h3>
            <ul className="mt-4 space-y-2">
              <li>
                <a href="#" className="text-sm text-surface-400 hover:text-white">
                  Privacy Policy
                </a>
              </li>
              <li>
                <a href="#" className="text-sm text-surface-400 hover:text-white">
                  Terms of Service
                </a>
              </li>
              <li>
                <a href="#" className="text-sm text-surface-400 hover:text-white">
                  Cookie Policy
                </a>
              </li>
            </ul>
          </div>
        </div>
        <div className="mt-12 border-t border-surface-800 pt-8 text-center">
          <p className="text-sm text-surface-500">
            &copy; {new Date().getFullYear()} Hanzo Industries, Inc. All rights reserved.
          </p>
        </div>
      </div>
    </footer>
  )
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en" className="dark">
      <body className="min-h-screen flex flex-col">
        <Navbar />
        <main className="flex-1">{children}</main>
        <Footer />
      </body>
    </html>
  )
}
