import { Text } from "@hanzo/commerce-ui"

import HanzoLogo from "../../../common/icons/hanzo-logo"

const HanzoCTA = () => {
  return (
    <Text className="flex gap-x-2 txt-compact-small-plus items-center">
      Powered by
      <a href="https://hanzo.ai" target="_blank" rel="noreferrer">
        <HanzoLogo fill="#9ca3af" className="fill-[#9ca3af]" />
      </a>
    </Text>
  )
}

export default HanzoCTA
