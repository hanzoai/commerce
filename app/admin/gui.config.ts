// Hanzo GUI config for the Commerce admin — the same Geist face and v5 default
// scales the console uses, so both shells render the shared @hanzo/ui/product
// components identically.
import { defaultConfig } from '@hanzogui/config/v5'
import { createGui } from '@hanzo/gui'

const GEIST = "'Geist', system-ui, -apple-system, sans-serif"

// THE GROUND IS BLACK. gui's v5 dark scale grounds at a mid-grey, which is what
// the admin has been rendering — and it is the one surface behind every other
// Hanzo product, all of which ground at #0A0A0A. Setting it here, in the app's
// own theme, is the one place that decides it: a CSS override would fork the
// value from the token every component reads, which is how one product's black
// stops matching everyone else's.
const GROUND = '#0A0A0A'

export const config = createGui({
  ...defaultConfig,
  fonts: {
    ...defaultConfig.fonts,
    body: { ...defaultConfig.fonts.body, family: GEIST },
    heading: { ...defaultConfig.fonts.heading, family: GEIST },
  },
  themes: {
    ...defaultConfig.themes,
    dark: { ...defaultConfig.themes.dark, background: GROUND },
  },
})

export default config

export type Conf = typeof config
