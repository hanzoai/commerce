import { readdirSync } from 'node:fs'
import { createRequire } from 'node:module'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import type { NextConfig } from 'next'

import { BASE_PATH } from './src/lib/basepath'

const HERE = path.dirname(fileURLToPath(import.meta.url))
const require_ = createRequire(import.meta.url)

/**
 * The file a package's own `exports` map names for `subpath` under the `import`
 * condition. 8.x Gui packages additionally ship legacy no-`exports` stubs (a
 * `v5/index.cjs` for Metro, which cannot read `exports`) whose body is a
 * package-relative `require('../dist/...')`; webpack picks the stub, externalizes
 * that inner relative request verbatim, and the emitted chunk then asks Node for
 * a path relative to ITSELF — the static export dies at prerender with "Cannot
 * find module '../dist/cjs/v5.cjs'". Asking the package which file it means is
 * the fix, and it stays correct as Gui moves its own files.
 */
function packageEntry(pkg: string, subpath: string): string {
  const manifestPath = require_.resolve(`${pkg}/package.json`)
  const entry = require_(manifestPath).exports[subpath].import as string
  return path.join(path.dirname(manifestPath), entry)
}

// `@hanzogui/config` is inert token data reached only through the `/v5` alias
// below (already a concrete file), so transpiling it buys nothing.
const NEVER_TRANSPILE = new Set(['@hanzogui/config'])

/**
 * Packages Next must run through its own transpile pass: the whole `@hanzogui/*`
 * set (read off disk, not hand-listed — one list that cannot drift as Gui splits
 * packages) plus the `@hanzo/*` ones that publish TS/TSX SOURCE rather than
 * compiled dist (`@hanzo/ui`, and `@hanzo/data`, which `@hanzo/ui/product`
 * imports). This is how the console spells it too.
 */
function guiPackages(): string[] {
  let scoped: string[] = []
  try {
    scoped = readdirSync(path.join(HERE, 'node_modules', '@hanzogui'))
      .map((n) => `@hanzogui/${n}`)
      .filter((p) => !NEVER_TRANSPILE.has(p))
  } catch {
    scoped = []
  }
  return ['@hanzo/gui', '@hanzo/ui', '@hanzo/data', '@hanzo/products', 'react-native-web', ...scoped]
}

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
 * aliases to `react-native-web` and `react-native-svg` to the Gui web shim. No
 * error suppression: `tsgo --noEmit` is a build gate (`turbo run typecheck build`).
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
  transpilePackages: guiPackages(),
  webpack(config) {
    // Pin the context-bearing packages to this app's copies (see SINGLETONS) so
    // a transitive copy can never become a second instance.
    config.resolve.alias = {
      ...config.resolve.alias,
      'react-native$': 'react-native-web',
      // Every 8.x Gui icon draws with react-native-svg primitives. On the web
      // those are plain SVG elements — @hanzogui/react-native-svg is the Gui
      // shim that spells them that way, and it is what the Gui compiler's own
      // vite/next plugins alias to. Aliasing here keeps the real (native-only,
      // Fabric-linked) package out of a browser bundle entirely.
      'react-native-svg$': '@hanzogui/react-native-svg',
      // The v5 token scale — the one config both this admin and the console build
      // on. Pinned to the file the package's `exports` map names (see packageEntry)
      // so its Metro compat stub never enters the graph.
      '@hanzogui/config/v5$': packageEntry('@hanzogui/config', './v5'),
      ...Object.fromEntries(SINGLETONS.map((p) => [p, path.join(HERE, 'node_modules', p)])),
    }
    config.resolve.extensions = ['.web.tsx', '.web.ts', '.web.jsx', '.web.js', ...config.resolve.extensions]
    return config
  },
}

export default config
