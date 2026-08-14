const { readdirSync } = require("node:fs")
const path = require("node:path")

const checkEnvVariables = require("./check-env-variables")

checkEnvVariables()

/**
 * The file a package's own `exports` map names for `subpath` under the `import`
 * condition. 8.x Gui packages also ship legacy no-`exports` stubs whose body is
 * a package-relative `require('../dist/...')`; webpack picks the stub, keeps
 * that inner relative request verbatim, and then asks Node for a path relative
 * to itself. Asking the package which file it means resolves it.
 */
function packageEntry(pkg, subpath) {
  const manifestPath = require.resolve(`${pkg}/package.json`)
  const entry = require(manifestPath).exports[subpath].import
  return path.join(path.dirname(manifestPath), entry)
}

// Token data, reached only through the `/v5` alias below.
const NEVER_TRANSPILE = new Set(["@hanzogui/config"])

/** Every @hanzogui/* package on disk, plus the @hanzo source packages. */
function guiPackages() {
  let scoped = []
  try {
    scoped = readdirSync(path.join(__dirname, "node_modules", "@hanzogui"))
      .map((n) => `@hanzogui/${n}`)
      .filter((p) => !NEVER_TRANSPILE.has(p))
  } catch {
    scoped = []
  }
  return [
    "@hanzo/gui",
    "@hanzo/ui",
    "@hanzo/commerce-ui",
    "react-native-web",
    ...scoped,
  ]
}

// Packages holding React context or module-level state that must be one
// instance: two copies of @hanzogui/web is two ThemeStateContexts, and the page
// dies with "Missing theme."
const SINGLETONS = [
  "@hanzo/gui",
  "@hanzogui/core",
  "@hanzogui/web",
  "@hanzogui/lucide-icons-2",
  "@hanzogui/next-theme",
]

/**
 * Cloud storage environment variables
 */
const S3_HOSTNAME = process.env.HANZO_COMMERCE_S3_HOSTNAME
const S3_PATHNAME = process.env.HANZO_COMMERCE_S3_PATHNAME

/**
 * @type {import('next').NextConfig}
 */
const nextConfig = {
  output: "standalone",
  reactStrictMode: true,
  logging: {
    fetches: {
      fullUrl: true,
    },
  },
  eslint: {
    ignoreDuringBuilds: true,
  },
  typescript: {
    ignoreBuildErrors: true,
  },
  transpilePackages: guiPackages(),
  webpack(config, { webpack }) {
    // @hanzo/ui's root imports its own styles.css from JS, which gives that
    // file a postcss pass of its own — and Tailwind refuses any sheet that
    // opens `@layer base` without a `@tailwind base` beside it. globals.css
    // @imports the same file instead, where postcss-import inlines it next to
    // the directives, so the bytes still reach the document exactly once. The
    // package's own guard reads `--hanzo-ui-styles`, which they declare.
    config.plugins.push(
      new webpack.IgnorePlugin({
        resourceRegExp: /^\.\/styles\.css$/,
        contextRegExp: /[\\/]@hanzo[\\/]ui[\\/]dist$/,
      })
    )
    config.resolve.alias = {
      ...config.resolve.alias,
      "react-native$": "react-native-web",
      // 8.x Gui icons draw with react-native-svg primitives; on the web those
      // are plain SVG elements, spelled by the Gui shim.
      "react-native-svg$": "@hanzogui/react-native-svg",
      // The v5 token scale, pinned to the file the package's exports map names.
      "@hanzogui/config/v5$": packageEntry("@hanzogui/config", "./v5"),
      ...Object.fromEntries(
        SINGLETONS.map((p) => [p, path.join(__dirname, "node_modules", p)])
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
  images: {
    remotePatterns: [
      {
        protocol: "http",
        hostname: "localhost",
      },
      {
        protocol: "https",
        hostname: "**.hanzo.ai",
      },
      ...(S3_HOSTNAME && S3_PATHNAME
        ? [
            {
              protocol: "https",
              hostname: S3_HOSTNAME,
              pathname: S3_PATHNAME,
            },
          ]
        : []),
    ],
  },
}

module.exports = nextConfig
