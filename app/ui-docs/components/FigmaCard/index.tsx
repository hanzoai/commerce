import React from "react"
import { Card } from "@hanzo/commerce-docs-ui"
import { basePathUrl } from "@/utils/base-path-url"

export const FigmaCard = () => {
  return (
    <Card
      title="Hanzo Commerce UI"
      text="Colors, type, icons and components"
      href="https://www.figma.com/community/file/1278648465968635936/Hanzo-Commerce-UI"
      image={basePathUrl("/images/figma.png")}
      iconClassName="!p-0"
    />
  )
}
