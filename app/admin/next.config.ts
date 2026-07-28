import path from 'node:path'
import type { NextConfig } from 'next'

import { BASE_PATH } from './src/lib/basepath'

// Packages that hold React context or module-level singletons and MUST be one
// instance. `@hanzo/ui` imports all of them and declares none — they are its
// devDependencies — so each consumer installs its own copy and webpack is free
// to resolve two. Two `@hanzogui/web`s is two ThemeStateContexts: `GuiProvider`
// publishes to one, every shared component reads the other, and the admin dies
// on load with "Missing theme." An alias applies to EVERY request, whoever the
// importer is, so pinning them to this app's copies is what makes the admin and
// the console render the same components off the same instance — and it also
// collapses copies pulled in transitively (e.g. `@hanzogui/next-theme`'s own
// `@hanzogui/core`). `@hanzogui/config` is deliberately absent: it is only ever
// imported by its `/v5` subpath, which a directory alias would resolve past the
// package's `exports` map — and it is inert data, registered through the one
// `@hanzo/gui` that IS pinned.
const SINGLETONS = ['@hanzo/gui', '@hanzogui/core', '@hanzogui/web', '@hanzogui/lucide-icons-2', '@hanzogui/next-theme']

/**
 * The Commerce admin — a static export baked into the commerced binary (ui/dist,
 * //go:embed) and served from its `/admin/*` mount.
 *
 * @hanzo/ui and @hanzo/gui ship ESM that still needs the app's own transpile pass,
 * so they are listed below exactly the way the console lists them; `react-native`
 * aliases to `react-native-web`. No error suppression: `tsc --noEmit` is a build
 * gate (`turbo run typecheck build`).
 */
const config: NextConfig = {
  output: 'export',
  // The export is mounted UNDER BASE_PATH, so it must be BUILT for it: basePath
  // prefixes both the chunk URLs (<base>/_next/*) and every route the client
  // router resolves. Without it Next emits root-absolute /_next/* and treats the
  // shell as living at "/", so every asset escapes the mount (404) and the router
  // 404s on its own pathname. One export, one URL contract.
  basePath: BASE_PATH,
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
    // Pin the context-bearing packages to this app's copies (see SINGLETONS) so
    // a transitive copy can never become a second instance.
    config.resolve.alias = {
      ...config.resolve.alias,
      'react-native$': 'react-native-web',
      ...Object.fromEntries(SINGLETONS.map((p) => [p, path.join(config.context, 'node_modules', p)])),
    }
    config.resolve.extensions = ['.web.tsx', '.web.ts', '.web.jsx', '.web.js', ...config.resolve.extensions]
    return config
  },
}

export default config
