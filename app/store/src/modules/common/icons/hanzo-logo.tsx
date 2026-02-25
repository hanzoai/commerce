import React from "react"

import { IconProps } from "types/icon"

const HanzoLogo: React.FC<IconProps> = ({
  size = "20",
  color = "#9CA3AF",
  ...attributes
}) => {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="18"
      height="18"
      viewBox="0 0 18 18"
      fill="none"
      {...attributes}
    >
      <rect x="1" y="1" width="16" height="16" rx="2" fill={color} />
      <rect x="4" y="4" width="4" height="4" rx="0.5" fill="white" />
      <rect x="10" y="4" width="4" height="4" rx="0.5" fill="white" />
      <rect x="4" y="10" width="4" height="4" rx="0.5" fill="white" />
      <rect x="10" y="10" width="4" height="4" rx="0.5" fill="white" />
    </svg>
  )
}

export default HanzoLogo
