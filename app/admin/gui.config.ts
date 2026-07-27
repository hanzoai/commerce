// Hanzo GUI config for the Commerce admin — the same Geist face and v5 default
// scales the console uses, so both shells render the shared @hanzo/ui/product
// components identically.
import { defaultConfig } from '@hanzogui/config/v5'
import { createGui } from '@hanzo/gui'

const GEIST = "'Geist', system-ui, -apple-system, sans-serif"

export const config = createGui({
  ...defaultConfig,
  fonts: {
    ...defaultConfig.fonts,
    body: { ...defaultConfig.fonts.body, family: GEIST },
    heading: { ...defaultConfig.fonts.heading, family: GEIST },
  },
})

export default config

export type Conf = typeof config
