import React from "react"

import { MARK_BLOCKS } from "@hanzo/logo/logos"

import { IconProps } from "types/icon"

/**
 * The Hanzo mark.
 *
 * This drew a four-square grid on a rounded tile — the upstream Medusa mark,
 * kept through the rename and rendered by the storefront's hanzo-cta. The
 * geometry comes from @hanzo/logo now, so it cannot drift from the mark every
 * other Hanzo surface serves.
 */
const HanzoLogo: React.FC<IconProps> = ({
  size = "20",
  color = "currentColor",
  ...attributes
}) => {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width={size}
      height={size}
      viewBox="0 0 67 67"
      fill="none"
      {...attributes}
    >
      {MARK_BLOCKS.map((d) => (
        <path key={d} d={d} fill={color} />
      ))}
    </svg>
  )
}

export default HanzoLogo
