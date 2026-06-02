const path = require("path")

// get the path of the dependency "@hanzo/commerce-ui"
const commerceUI = path.join(
  path.dirname(require.resolve("@hanzo/commerce-ui")),
  "**/*.{js,jsx,ts,tsx}"
)

/** @type {import('tailwindcss').Config} */
module.exports = {
  presets: [require("@hanzo/commerce-ui-preset")],
  content: ["./src/**/*.{js,ts,jsx,tsx}", commerceUI],
  darkMode: "class",
  theme: {
    extend: {},
  },
  plugins: [],
}
