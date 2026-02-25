import type { NextConfig } from 'next'

const config: NextConfig = {
  output: 'export',
  images: { unoptimized: true },
  typescript: {
    ignoreBuildErrors: true,
  },
  eslint: {
    ignoreDuringBuilds: true,
  },
  transpilePackages: [
    '@hanzo/commerce-ui',
    '@hanzo/commerce-icons',
    '@hanzo/commerce-sdk',
    '@hanzo/commerce-types',
    '@hanzo/commerce-admin-shared',
  ],
}

export default config
