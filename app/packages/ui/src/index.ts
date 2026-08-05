/**
 * @hanzo/commerce-ui — the storefront's component kit, on the 8.x stack.
 *
 * This was the vendored Medusa design system: 195 files on Radix, cva and the
 * Tailwind preset. It is now the 16 names the storefront actually renders,
 * drawn by @hanzo/ui on @hanzo/gui (interactive pieces) and by plain elements
 * on kit.css theme tokens (server-rendered pieces). The end state is call
 * sites importing @hanzo/ui directly as each file sheds its utility classes;
 * this module is the storefront's one seam until then.
 */
export {
  Badge,
  Container,
  Heading,
  IconBadge,
  Input,
  Label,
  Table,
  Text,
  clx,
} from "./server"

export {
  Button,
  Checkbox,
  IconButton,
  RadioGroup,
  Toaster,
  toast,
  useToggleState,
} from "./client"
