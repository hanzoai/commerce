/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./src/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        primary: {
          50: "#fff1f1",
          100: "#ffe0e0",
          200: "#ffc7c7",
          300: "#ffa0a0",
          400: "#fd4444",
          500: "#f52222",
          600: "#e20808",
          700: "#be0404",
          800: "#9d0808",
          900: "#820e0e",
          950: "#470202",
        },
        surface: {
          50: "#f7f7f8",
          100: "#eeeef0",
          200: "#d9d9de",
          300: "#b8b8c1",
          400: "#91919f",
          500: "#737384",
          600: "#5d5d6c",
          700: "#4c4c58",
          800: "#41414b",
          900: "#393941",
          950: "#0a0a0f",
        },
      },
      fontFamily: {
        sans: ["Inter", "system-ui", "-apple-system", "sans-serif"],
      },
    },
  },
  plugins: [],
}
