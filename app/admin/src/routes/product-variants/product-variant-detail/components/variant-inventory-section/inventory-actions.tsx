import { useTranslation } from "react-i18next"

import { Buildings } from "@hanzo/commerce-icons"
import { InventoryItemDTO } from "@hanzo/commerce-types"

import { ActionMenu } from "../../../../../components/common/action-menu"

export const InventoryActions = ({ item }: { item: InventoryItemDTO }) => {
  const { t } = useTranslation()

  return (
    <ActionMenu
      groups={[
        {
          actions: [
            {
              icon: <Buildings />,
              label: t("products.variant.inventory.navigateToItem"),
              to: `/inventory/${item.id}`,
            },
          ],
        },
      ]}
    />
  )
}
