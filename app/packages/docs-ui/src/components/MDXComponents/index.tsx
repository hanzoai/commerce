import React from "react"
import type { MDXComponents as MDXComponentsType } from "mdx/types"
import { CodeMdx } from "../CodeMdx"
import { Details, DetailsProps } from "../Details"
import { DetailsSummary } from "../Details/Summary"
import { Kbd } from "../Kbd"
import { Note } from "../Note"
import { Card } from "../Card"
import { CardList } from "../CardList"
import { ZoomImg } from "../ZoomImg"
import { H1 } from "../Heading/H1"
import { H2 } from "../Heading/H2"
import { H3 } from "../Heading/H3"
import { H4 } from "../Heading/H4"
import { Link } from "../Link"
import clsx from "clsx"
import { Text } from "@hanzo/commerce-ui"

export const MDXComponents: MDXComponentsType = {
  code: CodeMdx,
  kbd: Kbd,
  Kbd,
  Note,
  details: Details,
  Details: ({ className, ...props }: DetailsProps) => {
    return <Details {...props} className={clsx(className, "my-docs_1")} />
  },
  Summary: DetailsSummary,
  Card,
  CardList,
  h1: H1,
  h2: H2,
  h3: H3,
  h4: H4,
  p: ({ className, ...props }: React.HTMLAttributes<HTMLParagraphElement>) => {
    return (
      <p
        className={clsx(
          "text-hanzo-fg-base [&:not(:last-child)]:mb-docs_1.5 last:!mb-0",
          className
        )}
        {...props}
      />
    )
  },
  ul: ({
    className,
    children,
    ...props
  }: React.HTMLAttributes<HTMLUListElement>) => {
    return (
      <ul
        {...props}
        className={clsx(
          "list-disc px-docs_1 mb-docs_1.5 [&_ul]:mb-0 [&_ol]:mb-0 [&_p]:!mb-0",
          className
        )}
      >
        {children}
      </ul>
    )
  },
  ol: ({
    className,
    children,
    ...props
  }: React.HTMLAttributes<HTMLOListElement>) => {
    return (
      <ol
        {...props}
        className={clsx(
          "list-decimal px-docs_1 mb-docs_1.5 [&_ul]:mb-0 [&_ol]:mb-0 [&_p]:!mb-0",
          className
        )}
      >
        {children}
      </ol>
    )
  },
  li: ({
    className,
    children,
    ...props
  }: React.HTMLAttributes<HTMLElement>) => {
    return (
      <li
        className={clsx(
          "text-hanzo-fg-base [&:not(:last-child)]:mb-docs_0.5",
          "[&_ol]:mt-docs_0.5 [&_ul]:mt-docs_0.5",
          className
        )}
        {...props}
      >
        <Text as="span">{children}</Text>
      </li>
    )
  },
  hr: ({ className, ...props }: React.HTMLAttributes<HTMLHRElement>) => {
    return (
      <hr
        className={clsx(
          "my-docs_2 h-[1px] w-full border-0 bg-hanzo-border-base",
          className
        )}
        {...props}
      />
    )
  },
  img: (
    props: React.DetailedHTMLProps<
      React.ImgHTMLAttributes<HTMLImageElement>,
      HTMLImageElement
    >
  ) => {
    // omit key to resolve errors
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const { key, ...rest } = props
    return <ZoomImg {...rest} />
  },
  a: (props) => <Link {...props} variant="content" />,
  strong: ({ className, ...props }: React.HTMLAttributes<HTMLElement>) => {
    return <strong className={clsx("txt-medium-plus", className)} {...props} />
  },
}

export const Hr = MDXComponents["hr"] as () => React.JSX.Element
