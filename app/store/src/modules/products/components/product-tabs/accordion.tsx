import {
  Accordion as Root,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
  Text,
} from "@hanzo/ui"
import type { ComponentProps, FC, ReactNode } from "react"

/**
 * The product tabs, on the kit's Accordion.
 *
 * Everything this file used to hand-build — the open/closed chevron, the
 * heading level, `aria-expanded`/`aria-controls`, the roving arrow-key focus —
 * belongs to `@hanzo/ui`'s Accordion. What it drew instead was a MorphingTrigger
 * of two absolutely-positioned 1.5px bars rotated by `group-radix-state-open:`,
 * a `tailwindcss-radix` variant that only existed while Radix did. The trigger
 * is the primitive's now, so the plugin and the hand-drawn cross both go.
 */
type ItemProps = ComponentProps<typeof AccordionItem> & {
  title: string
  subtitle?: string
  description?: string
  children: ReactNode
}

const Item: FC<ItemProps> = ({
  title,
  subtitle,
  description,
  children,
  ...props
}) => (
  <AccordionItem {...props}>
    <AccordionTrigger headingLevel={3}>
      <Text>{title}</Text>
      {subtitle && <Text opacity={0.6}>{subtitle}</Text>}
    </AccordionTrigger>
    <AccordionContent>
      {description && <Text>{description}</Text>}
      {children}
    </AccordionContent>
  </AccordionItem>
)

const Accordion = Object.assign(Root, { Item })

export default Accordion
