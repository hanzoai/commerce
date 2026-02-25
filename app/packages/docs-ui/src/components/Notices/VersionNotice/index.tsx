import React from "react"
import { Badge } from "../../Badge"
import { Tooltip } from "../../Tooltip"

export type VersionNoticeProps = {
  version: string
  tooltipTextClassName?: string
  badgeClassName?: string
  badgeContent?: React.ReactNode
}

export const VersionNotice = ({
  version,
  tooltipTextClassName,
  badgeClassName,
  badgeContent = `v${version}`,
}: VersionNoticeProps) => {
  return (
    <Tooltip
      tooltipChildren={
        <span className={tooltipTextClassName}>
          This is available starting from <br />
          <a
            href={`https://github.com/hanzoai/commerce/releases/tag/${version}`}
          >
            Hanzo Commerce v{version}
          </a>
        </span>
      }
      clickable
    >
      <Badge variant="blue" className={badgeClassName}>
        {badgeContent}
      </Badge>
    </Tooltip>
  )
}
