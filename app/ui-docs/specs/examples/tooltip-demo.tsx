import { InformationCircleSolid } from "@hanzo/commerce-icons"
import { Tooltip } from "@hanzo/commerce-ui"

export default function TooltipDemo() {
  return (
    <Tooltip content="The quick brown fox jumps over the lazy dog.">
      <InformationCircleSolid />
    </Tooltip>
  )
}
