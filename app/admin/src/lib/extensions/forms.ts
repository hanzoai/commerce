import type { FormModule } from "../../dashboard-app/types"
import type { CustomFieldModel } from "@hanzo/commerce-admin-shared"

const formModule: FormModule = {
  customFields: {} as Record<CustomFieldModel, { forms: []; configs: [] }>,
}

export default formModule
