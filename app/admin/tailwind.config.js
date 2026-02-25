/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './src/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        hanzo: {
          red: '#fd4444',
          'red-dark': '#d93636',
          'red-light': '#ff6b6b',
        },
        surface: {
          DEFAULT: '#0a0a0a',
          raised: '#141414',
          overlay: '#1c1c1c',
        },
        border: {
          DEFAULT: '#262626',
          hover: '#3a3a3a',
        },
        muted: '#737373',
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', '-apple-system', 'sans-serif'],
      },
    },
  },
  plugins: [],
}
