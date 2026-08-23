// The Commerce admin renders @hanzo/ui/product, so it renders that kit's scale.
// @hanzo/ui/gui-config IS the scale — the Geist face, the type/radius/spacing
// ladder and the #0A0A0A dark ground the console and every other Hanzo surface
// draw on. This file used to restate it from @hanzogui/config/v5's defaults,
// which is a second copy of a number: the kit moves, the admin does not, and
// the two shells stop matching. Re-exporting is what keeps them one.
export { config, config as default } from '@hanzo/ui/gui-config'

import type { config as shared } from '@hanzo/ui/gui-config'

export type Conf = typeof shared
