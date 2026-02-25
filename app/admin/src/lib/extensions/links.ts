import type { LinkModule } from "../../dashboard-app/types"
import type { CustomFieldModel } from "@hanzo/commerce-admin-shared"

const linkModule: LinkModule = {
  links: {} as Record<CustomFieldModel, (string | string[])[]>,
}

export default linkModule
