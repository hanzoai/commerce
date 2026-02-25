import type { NextConfig } from 'next'

const BACKEND_URL =
  process.env.NEXT_PUBLIC_HANZO_COMMERCE_BACKEND_URL || "https://api.commerce.hanzo.ai"
const STOREFRONT_URL =
  process.env.NEXT_PUBLIC_HANZO_COMMERCE_STOREFRONT_URL || "http://localhost:8000"
const BASE = process.env.NEXT_PUBLIC_HANZO_COMMERCE_BASE || "/"
const AUTH_TYPE = process.env.NEXT_PUBLIC_HANZO_COMMERCE_AUTH_TYPE || "session"
const JWT_TOKEN_STORAGE_KEY = process.env.NEXT_PUBLIC_HANZO_COMMERCE_JWT_TOKEN_STORAGE_KEY || ""
const MAX_UPLOAD_FILE_SIZE = process.env.NEXT_PUBLIC_HANZO_COMMERCE_MAX_UPLOAD_FILE_SIZE || ""

const config: NextConfig = {
  output: 'export',
  images: { unoptimized: true },
  transpilePackages: [
    '@hanzo/commerce-ui',
    '@hanzo/commerce-icons',
    '@hanzo/commerce-sdk',
    '@hanzo/commerce-types',
    '@hanzo/commerce-admin-shared',
  ],
  env: {
    __BACKEND_URL__: BACKEND_URL,
    __STOREFRONT_URL__: STOREFRONT_URL,
    __BASE__: BASE,
    __AUTH_TYPE__: AUTH_TYPE,
    __JWT_TOKEN_STORAGE_KEY__: JWT_TOKEN_STORAGE_KEY,
    __MAX_UPLOAD_FILE_SIZE__: MAX_UPLOAD_FILE_SIZE,
  },
}

export default config
