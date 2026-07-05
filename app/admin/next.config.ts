import type { NextConfig } from 'next'

// Build-time config is read directly from NEXT_PUBLIC_HANZO_COMMERCE_* env vars
// at the point of use (Next inlines NEXT_PUBLIC_* into the client bundle). The
// prior `env: { __X__ }` mapping is invalid on Next 15 (keys may not start with
// `__`) and is unnecessary.
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
}

export default config
