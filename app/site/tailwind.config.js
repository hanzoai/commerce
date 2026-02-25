/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: "class",
  content: [
    "./app/**/*.{js,ts,jsx,tsx}",
    "./components/**/*.{js,ts,jsx,tsx}",
    "./providers/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        brand: {
          DEFAULT: "#fd4444",
          foreground: "#ffffff",
          50: "#fff1f1",
          100: "#ffe0e0",
          200: "#ffc7c7",
          300: "#ffa0a0",
          400: "#fd4444",
          500: "#f52222",
          600: "#e20808",
          700: "#be0404",
          800: "#9d0808",
          900: "#820f0f",
        },
      },
      fontFamily: {
        sans: ["Inter", "system-ui", "-apple-system", "sans-serif"],
        mono: ["Roboto Mono", "ui-monospace", "monospace"],
      },
      container: {
        center: true,
        screens: {
          "2xl": "1400px",
        },
      },
    },
  },
  plugins: [],
}
