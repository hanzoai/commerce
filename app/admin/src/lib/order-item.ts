import { OrderLineItemDTO } from "@hanzo/commerce-types"

export const getFulfillableQuantity = (item: OrderLineItemDTO) => {
  return item.quantity - item.detail.fulfilled_quantity
}
