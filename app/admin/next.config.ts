import type { NextConfig } from 'next'

/**
 * The Commerce admin — a static export served by hanzoai/static.
 *
 * @hanzo/ui and @hanzo/gui ship raw TS/ESM source, so they are transpiled here the
 * same way the console does it; `react-native` aliases to `react-native-web`. No
 * error suppression: `tsc --noEmit` is a build gate (`turbo run typecheck build`).
 */
const config: NextConfig = {
  output: 'export',
  // Directory-style export (out/overview/index.html) so a deep-link / refresh on a
  // route resolves via the static server's directory-index.
  trailingSlash: true,
  images: { unoptimized: true },
  transpilePackages: [
    '@hanzo/ui',
    '@hanzo/gui',
    '@hanzo/products',
    '@hanzogui/config',
    '@hanzogui/core',
    '@hanzogui/lucide-icons-2',
    '@hanzogui/web',
    'react-native-web',
  ],
  webpack(config) {
    // @hanzo/ui is consumed from SOURCE via a workspace link. Keep the symlinked
    // path so its own imports (@hanzo/gui, @hanzogui/*) walk up into THIS app's
    // node_modules — one Tamagui instance, matching the tsconfig `paths` pins.
    config.resolve.symlinks = false
    config.resolve.alias = { ...config.resolve.alias, 'react-native$': 'react-native-web' }
    config.resolve.extensions = ['.web.tsx', '.web.ts', '.web.jsx', '.web.js', ...config.resolve.extensions]
    return config
  },
}

export default config
