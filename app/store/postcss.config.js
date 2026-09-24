const { createRequire } = require("node:module")

const req = createRequire(__filename)

module.exports = {
  plugins: {
    // Inline every @import first, so the whole sheet is one file by the time
    // Tailwind runs. Without it css-loader hands each imported file its own
    // postcss pass, and a shared stylesheet that opens `@layer base` is read
    // with no `@tailwind base` in sight.
    "postcss-import": {
      // postcss-import's own resolver predates package `exports`, so a
      // published stylesheet named through an exports map (@hanzo/ui/theme.css)
      // is invisible to it. Node knows where those live; bare paths like
      // `tailwindcss/base` are not Node-resolvable and stay with the default.
      resolve(id) {
        if (id.startsWith(".") || id.startsWith("/")) return id
        try {
          return req.resolve(id)
        } catch {
          return id
        }
      },
    },
    tailwindcss: {},
    autoprefixer: {},
  },
}
