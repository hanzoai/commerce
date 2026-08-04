"use client"

import Back from "@modules/common/icons/back"
import FastDelivery from "@modules/common/icons/fast-delivery"
import Refresh from "@modules/common/icons/refresh"

import { HttpTypes } from "@hanzo/commerce-types"

type ProductTabsProps = {
  product: HttpTypes.StoreProduct
}

/**
 * One independently-openable panel. `<details>` is the browser's own disclosure
 * widget — open/closed state, keyboard operation and assistive-tech semantics
 * come with the element — and several of them side by side already behave the
 * way a multi-open accordion does, so there is nothing left for a component
 * library to contribute. The marker is a plus that rotates into a minus.
 */
const Panel = ({
  label,
  children,
}: {
  label: string
  children: React.ReactNode
}) => (
  <details className="group border-grey-20 border-t py-3 last:mb-0 last:border-b">
    <summary className="flex w-full cursor-pointer list-none items-center justify-between px-1 [&::-webkit-details-marker]:hidden">
      <span className="text-ui-fg-subtle text-sm">{label}</span>
      <span className="text-grey-90 hover:bg-grey-5 rounded p-[6px]">
        <span className="relative block h-5 w-5">
          <span className="bg-grey-50 absolute inset-y-[31.75%] left-[48%] right-1/2 w-[1.5px] duration-300 group-open:rotate-90" />
          <span className="bg-grey-50 absolute inset-x-[31.75%] top-[48%] bottom-1/2 h-[1.5px]" />
        </span>
      </span>
    </summary>
    <div className="px-1">{children}</div>
  </details>
)

const ProductTabs = ({ product }: ProductTabsProps) => {
  return (
    <div className="w-full">
      <Panel label="Product Information">
        <ProductInfoTab product={product} />
      </Panel>
      <Panel label="Shipping &amp; Returns">
        <ShippingInfoTab />
      </Panel>
    </div>
  )
}

const ProductInfoTab = ({ product }: ProductTabsProps) => {
  return (
    <div className="text-small-regular py-8">
      <div className="grid grid-cols-2 gap-x-8">
        <div className="flex flex-col gap-y-4">
          <div>
            <span className="font-semibold">Material</span>
            <p>{product.material ? product.material : "-"}</p>
          </div>
          <div>
            <span className="font-semibold">Country of origin</span>
            <p>{product.origin_country ? product.origin_country : "-"}</p>
          </div>
          <div>
            <span className="font-semibold">Type</span>
            <p>{product.type ? product.type.value : "-"}</p>
          </div>
        </div>
        <div className="flex flex-col gap-y-4">
          <div>
            <span className="font-semibold">Weight</span>
            <p>{product.weight ? `${product.weight} g` : "-"}</p>
          </div>
          <div>
            <span className="font-semibold">Dimensions</span>
            <p>
              {product.length && product.width && product.height
                ? `${product.length}L x ${product.width}W x ${product.height}H`
                : "-"}
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}

const ShippingInfoTab = () => {
  return (
    <div className="text-small-regular py-8">
      <div className="grid grid-cols-1 gap-y-8">
        <div className="flex items-start gap-x-2">
          <FastDelivery />
          <div>
            <span className="font-semibold">Fast delivery</span>
            <p className="max-w-sm">
              Your package will arrive in 3-5 business days at your pick up
              location or in the comfort of your home.
            </p>
          </div>
        </div>
        <div className="flex items-start gap-x-2">
          <Refresh />
          <div>
            <span className="font-semibold">Simple exchanges</span>
            <p className="max-w-sm">
              Is the fit not quite right? No worries - we&apos;ll exchange your
              product for a new one.
            </p>
          </div>
        </div>
        <div className="flex items-start gap-x-2">
          <Back />
          <div>
            <span className="font-semibold">Easy returns</span>
            <p className="max-w-sm">
              Just return your product and we&apos;ll refund your money. No
              questions asked – we&apos;ll do our best to make sure your return
              is hassle-free.
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}

export default ProductTabs
