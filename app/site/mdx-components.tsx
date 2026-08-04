import type { MDXComponents } from "mdx/types"

import * as docs from "@/components/docs"

/**
 * The components every MDX page can name — supplied once, here.
 *
 * MDX resolves a capitalised tag it cannot see in module scope through this
 * provider, so a page writes `<Note>` or `<Prerequisites/>` and gets the kit
 * without an import line. That is why the kit is the ONE definition of these
 * shapes: there is no second way for a page to obtain a different `Table`.
 *
 * The raw elements MDX emits (h1, p, ul, pre, …) are not overridden — they are
 * styled by the `.docs-prose` rules in `app/globals.css`, plain CSS on the Hanzo
 * tokens, because no component wraps them.
 *
 * This file is required to use MDX in the `app` directory.
 */
export function useMDXComponents(components: MDXComponents): MDXComponents {
  return { ...docs, ...components }
}
