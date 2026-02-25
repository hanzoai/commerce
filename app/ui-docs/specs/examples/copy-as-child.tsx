import { PlusMini } from "@hanzo/commerce-icons"
import { Copy, IconButton, Text } from "@hanzo/commerce-ui"

export default function CopyAsChild() {
  return (
    <div className="flex items-center gap-x-2">
      <Text>Copy command</Text>
      <Copy content="yarn add @hanzo/commerce-ui" asChild>
        <IconButton>
          <PlusMini />
        </IconButton>
      </Copy>
    </div>
  )
}
