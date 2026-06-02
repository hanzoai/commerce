import mdx from "@next/mdx"
import rehypeMdxCodeProps from "rehype-mdx-code-props"
import rehypeSlug from "rehype-slug"
import remarkFrontmatter from "remark-frontmatter"

const withMDX = mdx({
  extension: /\.mdx?$/,
  options: {
    rehypePlugins: [
      [
        rehypeMdxCodeProps,
        {
          tagName: "code",
        },
      ],
      [rehypeSlug],
    ],
    remarkPlugins: [[remarkFrontmatter]],
    jsx: true,
  },
})

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "export",
  pageExtensions: ["js", "jsx", "mdx", "ts", "tsx"],
  transpilePackages: ["@hanzo/commerce-docs-ui"],
  images: {
    unoptimized: true,
  },
  experimental: {
    optimizePackageImports: ["@hanzo/commerce-icons", "@hanzo/commerce-ui"],
  },
}

export default withMDX(nextConfig)
