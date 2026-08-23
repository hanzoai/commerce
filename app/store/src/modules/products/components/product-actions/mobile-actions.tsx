import { Dialog, DialogContent } from "@hanzo/ui"
import { Grid } from "@hanzo/ui/grid"
import { Button, clx } from "@hanzo/commerce-ui"
import React, { useMemo } from "react"

import useToggleState from "@lib/hooks/use-toggle-state"
import ChevronDown from "@modules/common/icons/chevron-down"
import X from "@modules/common/icons/x"

import { getProductPrice } from "@lib/util/get-product-price"
import OptionSelect from "./option-select"
import { HttpTypes } from "@hanzo/commerce-types"
import { isSimpleProduct } from "@lib/util/product"

type MobileActionsProps = {
  product: HttpTypes.StoreProduct
  variant?: HttpTypes.StoreProductVariant
  options: Record<string, string | undefined>
  updateOptions: (title: string, value: string) => void
  inStock?: boolean
  handleAddToCart: () => void
  isAdding?: boolean
  show: boolean
  optionsDisabled: boolean
}

const MobileActions: React.FC<MobileActionsProps> = ({
  product,
  variant,
  options,
  updateOptions,
  inStock,
  handleAddToCart,
  isAdding,
  show,
  optionsDisabled,
}) => {
  const { state, open, close } = useToggleState()

  const price = getProductPrice({
    product: product,
    variantId: variant?.id,
  })

  const selectedPrice = useMemo(() => {
    if (!price) {
      return null
    }
    const { variantPrice, cheapestPrice } = price

    return variantPrice || cheapestPrice || null
  }, [price])

  const isSimple = isSimpleProduct(product)

  return (
    <>
      <div
        className={clx("lg:hidden inset-x-0 bottom-0 fixed z-50", {
          "pointer-events-none": !show,
        })}
      >
        <div
          className="bg-white grid gap-y-3 justify-center items-center text-large-regular p-4 h-full w-full border-t border-gray-200"
          style={{ opacity: show ? 1 : 0, transition: "opacity 300ms ease-in-out" }}
          data-testid="mobile-actions"
        >
            <div className="flex items-center gap-x-2">
              <span data-testid="mobile-title">{product.title}</span>
              <span>—</span>
              {selectedPrice ? (
                <div className="flex items-end gap-x-2 text-ui-fg-base">
                  {selectedPrice.price_type === "sale" && (
                    <p>
                      <span className="line-through text-small-regular">
                        {selectedPrice.original_price}
                      </span>
                    </p>
                  )}
                  <span
                    className={clx({
                      "text-ui-fg-interactive":
                        selectedPrice.price_type === "sale",
                    })}
                  >
                    {selectedPrice.calculated_price}
                  </span>
                </div>
              ) : (
                <div></div>
              )}
            </div>
            <Grid columns={isSimple ? 1 : 2} gap={16} style={{ width: "100%" }}>
              {!isSimple && <Button
                onClick={open}
                variant="secondary"
                className="w-full"
                data-testid="mobile-actions-button"
              >
                <div className="flex items-center justify-between w-full">
                  <span>
                    {variant
                      ? Object.values(options).join(" / ")
                      : "Select Options"}
                  </span>
                  <ChevronDown />
                </div>
              </Button>}
              <Button
                onClick={handleAddToCart}
                disabled={!inStock || !variant}
                className="w-full"
                isLoading={isAdding}
                data-testid="mobile-cart-button"
              >
                {!variant
                  ? "Select variant"
                  : !inStock
                  ? "Out of stock"
                  : "Add to cart"}
              </Button>
            </Grid>
          </div>
      </div>
      {/* The option picker. Its overlay, its two enter/leave transitions and its
          focus trap were three Headless UI wrappers deep; DialogContent is all
          three. The bottom-sheet placement is the only thing left to say. */}
      <Dialog modal open={state} onOpenChange={(o) => !o && close()}>
        <DialogContent
          data-testid="mobile-actions-modal"
          showCloseButton={false}
          p={0}
          gap="$3"
          style={{
            position: "fixed",
            bottom: 0,
            left: 0,
            right: 0,
            top: "auto",
            maxWidth: "100%",
            width: "100%",
            transform: "none",
          }}
        >
          <div className="w-full flex justify-end pr-6">
            <button
              onClick={close}
              className="bg-white w-12 h-12 rounded-full text-ui-fg-base flex justify-center items-center"
              data-testid="close-modal-button"
            >
              <X />
            </button>
          </div>
          <div className="bg-white px-6 py-12">
            {(product.variants?.length ?? 0) > 1 && (
              <div className="grid gap-y-6">
                {(product.options || []).map((option) => (
                  <OptionSelect
                    key={option.id}
                    option={option}
                    current={options[option.id]}
                    updateOption={updateOptions}
                    title={option.title ?? ""}
                    disabled={optionsDisabled}
                  />
                ))}
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}

export default MobileActions
