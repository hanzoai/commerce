import { Tooltip } from "@hanzo/commerce-ui"
import {
  ArrowLongDown,
  ArrowLongLeft,
  ArrowLongRight,
  ArrowLongUp,
} from "@hanzo/commerce-icons"

export default function TooltipSides() {
  return (
    <div className="flex gap-8 items-center justify-center">
      <Tooltip content="Top" side="top">
        <ArrowLongUp />
      </Tooltip>
      <Tooltip content="Bottom" side="bottom">
        <ArrowLongDown />
      </Tooltip>
      <Tooltip content="Left" side="left">
        <ArrowLongLeft />
      </Tooltip>
      <Tooltip content="Right" side="right">
        <ArrowLongRight />
      </Tooltip>
    </div>
  )
}
