import type { NextConfig } from 'next'

const config: NextConfig = {
  // Static SPA — go:embed'd into the commerce binary, served at /admin.
  // No Node runtime in production: hanzoai/spa pattern.
  output: 'export',
  trailingSlash: true,
  basePath: '/admin',
  assetPrefix: '/admin',
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
