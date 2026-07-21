const c = require("ansi-colors")

const requiredEnvs = [
  {
    key: "NEXT_PUBLIC_HANZO_COMMERCE_KEY",
    description:
      "A publishable API key for your Hanzo Commerce store.",
  },
]

function checkEnvVariables() {
  // Build-time skip: the storefront's publishable key is resolved PER-TENANT at
  // runtime (store config keyed by space_id, resolved per-request), so it is not
  // a build-time constant. CI/Docker `next build` sets SKIP_ENV_VALIDATION=1 to
  // pass; the key is validated where it actually matters — at request time.
  if (process.env.SKIP_ENV_VALIDATION) return

  const missingEnvs = requiredEnvs.filter(function (env) {
    return !process.env[env.key] && !(env.fallback && process.env[env.fallback])
  })

  if (missingEnvs.length > 0) {
    console.error(
      c.red.bold("\n🚫 Error: Missing required environment variables\n")
    )

    missingEnvs.forEach(function (env) {
      console.error(c.yellow(`  ${c.bold(env.key)}`))
      if (env.description) {
        console.error(c.dim(`    ${env.description}\n`))
      }
    })

    console.error(
      c.yellow(
        "\nPlease set these variables in your .env file or environment before starting the application.\n"
      )
    )

    process.exit(1)
  }
}

module.exports = checkEnvVariables
