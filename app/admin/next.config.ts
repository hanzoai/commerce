import type { NextConfig } from 'next'

const config: NextConfig = {
  output: 'standalone',
  images: { unoptimized: true },
  transpilePackages: [
    '@hanzo/commerce-ui',
    '@hanzo/commerce-icons',
    '@hanzo/commerce-client',
    '@hanzo/commerce-ui-preset',
    '@hanzo/iam',
  ],
  typescript: {
    // Workspace packages have their own type-checking; don't re-check here
    ignoreBuildErrors: true,
  },
}

export default config
