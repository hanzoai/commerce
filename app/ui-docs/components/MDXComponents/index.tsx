import type { MDXComponents as MDXComponentsType } from "mdx/types"
import {
  MDXComponents as UiMdxComponents,
  InlineThemeImage,
  InlineIcon,
} from "@hanzo/commerce-docs-ui"

const MDXComponents: MDXComponentsType = {
  ...UiMdxComponents,
  InlineThemeImage,
  InlineIcon,
}

export default MDXComponents
