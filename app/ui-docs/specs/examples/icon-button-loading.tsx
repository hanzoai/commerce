import { PlusMini } from "@hanzo/commerce-icons"
import { IconButton } from "@hanzo/commerce-ui"

export default function IconButtonLoading() {
  return (
    <IconButton isLoading className="relative">
      <PlusMini />
    </IconButton>
  )
}
