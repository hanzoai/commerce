import { readdirSync } from "node:fs"
import { createRequire } from "node:module"
import path from "node:path"
import { fileURLToPath } from "node:url"
import mdx from "@next/mdx"
import rehypeMdxCodeProps from "rehype-mdx-code-props"
import rehypeSlug from "rehype-slug"
import remarkFrontmatter from "remark-frontmatter"

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
function packageEntry(pkg, subpath) {
  const manifestPath = require_.resolve(`${pkg}/package.json`)
  const entry = require_(manifestPath).exports[subpath].import
  return path.join(path.dirname(manifestPath), entry)
}

// `@hanzogui/config` is inert token data reached only through the `/v5` alias
// below (already a concrete file), so transpiling it buys nothing.
const NEVER_TRANSPILE = new Set(["@hanzogui/config"])

/**
 * Packages Next must run through its own transpile pass: the whole `@hanzogui/*`
 * set (read off disk, not hand-listed — one list that cannot drift as Gui splits
 * packages) plus the `@hanzo/*` ones that publish TS/TSX SOURCE rather than
 * compiled dist. This is how the Commerce admin and the console spell it too.
 */
function guiPackages() {
  let scoped = []
  try {
    scoped = readdirSync(path.join(HERE, "node_modules", "@hanzogui"))
      .map((n) => `@hanzogui/${n}`)
      .filter((p) => !NEVER_TRANSPILE.has(p))
  } catch {
    scoped = []
  }
  return [
    "@hanzo/gui",
    "@hanzo/ui",
    "@hanzo/data",
    "@hanzo/products",
    "react-native-web",
    ...scoped,
  ]
}

// Packages that hold React context or module-level singletons and MUST be one
// instance. Two `@hanzogui/web`s is two ThemeStateContexts: `GuiProvider`
// publishes to one, every shared component reads the other, and the page dies on
// load with "Missing theme." See app/admin/next.config.ts for the long form.
const SINGLETONS = [
  "@hanzo/gui",
  "@hanzogui/core",
  "@hanzogui/web",
  "@hanzogui/lucide-icons-2",
  "@hanzogui/next-theme",
]

const withMDX = mdx({
  extension: /\.mdx?$/,
  options: {
    rehypePlugins: [
      [
        rehypeMdxCodeProps,
        {
          tagName: "code",
        },
      ],
      [rehypeSlug],
    ],
    remarkPlugins: [[remarkFrontmatter]],
    jsx: true,
  },
})

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "export",
  // Directory-style export (out/learn/index.html) so a deep link or a refresh on
  // a route resolves through the static server's directory index. Without it the
  // export writes out/learn.html, which /learn does not reach — the same reason
  // the admin sets it.
  trailingSlash: true,
  pageExtensions: ["js", "jsx", "mdx", "ts", "tsx"],
  images: {
    unoptimized: true,
  },
  transpilePackages: guiPackages(),
  experimental: {
    optimizePackageImports: ["@hanzo/commerce-icons"],
  },
  webpack(config) {
    config.resolve.alias = {
      ...config.resolve.alias,
      "react-native$": "react-native-web",
      // Every 8.x Gui icon draws with react-native-svg primitives. On the web
      // those are plain SVG elements — @hanzogui/react-native-svg is the Gui
      // shim that spells them that way, and it is what the Gui compiler's own
      // vite/next plugins alias to.
      "react-native-svg$": "@hanzogui/react-native-svg",
      // The v5 token scale — the one config this site, the admin and the console
      // all build on. Pinned to the file the package's `exports` map names (see
      // packageEntry) so its Metro compat stub never enters the graph.
      "@hanzogui/config/v5$": packageEntry("@hanzogui/config", "./v5"),
      ...Object.fromEntries(
        SINGLETONS.map((p) => [p, path.join(HERE, "node_modules", p)])
      ),
    }
    config.resolve.extensions = [
      ".web.tsx",
      ".web.ts",
      ".web.jsx",
      ".web.js",
      ...config.resolve.extensions,
    ]
    return config
  },
}

export default withMDX(nextConfig)
