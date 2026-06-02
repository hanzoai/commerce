import type { DisplayModule } from "../../dashboard-app/types"
import type { CustomFieldModel } from "@hanzo/commerce-admin-shared"

const displayModule: DisplayModule = {
  displays: {} as Record<CustomFieldModel, []>,
}

export default displayModule
