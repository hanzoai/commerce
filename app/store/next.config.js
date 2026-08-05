const { readdirSync } = require("node:fs")
const path = require("node:path")

const checkEnvVariables = require("./check-env-variables")

checkEnvVariables()

/**
 * The file a package's own `exports` map names for `subpath` under the `import`
 * condition. 8.x Gui packages additionally ship legacy no-`exports` stubs whose
 * body is a package-relative `require('../dist/...')`; webpack picks the stub,
 * externalizes that inner relative request verbatim, and the build then asks
 * Node for a path relative to ITSELF. Asking the package which file it means is
 * the fix — same as app/site and app/admin.
 */
function packageEntry(pkg, subpath) {
  const manifestPath = require.resolve(`${pkg}/package.json`)
  const entry = require(manifestPath).exports[subpath].import
  return path.join(path.dirname(manifestPath), entry)
}

// Inert token data reached only through the `/v5` alias below.
const NEVER_TRANSPILE = new Set(["@hanzogui/config"])

/** Every @hanzogui/* package read off disk, plus the @hanzo source packages. */
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

// Packages that hold React context or module-level singletons and MUST be one
// instance — two @hanzogui/web is two ThemeStateContexts and the page dies with
// "Missing theme." See app/admin/next.config.ts for the long form.
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
  webpack(config) {
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
