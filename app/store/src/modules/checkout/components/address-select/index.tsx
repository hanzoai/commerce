import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@hanzo/ui"
import { useMemo } from "react"

import compareAddresses from "@lib/util/compare-addresses"
import { HttpTypes } from "@hanzo/commerce-types"

type AddressSelectProps = {
  addresses: HttpTypes.StoreCustomerAddress[]
  addressInput: HttpTypes.StoreCartAddress | null
  onSelect: (
    address: HttpTypes.StoreCartAddress | undefined,
    email?: string
  ) => void
}

/**
 * Saved addresses, on the kit's Select.
 *
 * The chevron that rotated on open, the leave transition, and the dot marking
 * the chosen row were three hand-built parts of what a Select already is — and
 * the dot in particular was `@modules/common/components/radio`, which hardcodes
 * `aria-checked="true"`, so every saved address announced itself as selected.
 * SelectItem marks the current row itself.
 */
const AddressSelect = ({
  addresses,
  addressInput,
  onSelect,
}: AddressSelectProps) => {
  const handleSelect = (id: string) => {
    const savedAddress = addresses.find((a) => a.id === id)
    if (savedAddress) {
      onSelect(savedAddress as HttpTypes.StoreCartAddress)
    }
  }

  const selectedAddress = useMemo(
    () => addresses.find((a) => compareAddresses(a, addressInput)),
    [addresses, addressInput]
  )

  return (
    <Select value={selectedAddress?.id} onValueChange={handleSelect}>
      <SelectTrigger data-testid="shipping-address-select">
        <SelectValue placeholder="Choose an address">
          {selectedAddress?.address_1 ?? "Choose an address"}
        </SelectValue>
      </SelectTrigger>
      <SelectContent data-testid="shipping-address-options">
        {addresses.map((address, index) => (
          <SelectItem
            key={address.id}
            index={index}
            value={address.id}
            data-testid="shipping-address-option"
          >
            <div className="grid text-left">
              <span className="text-base-semi">
                {address.first_name} {address.last_name}
              </span>
              {address.company && (
                <span className="text-small-regular text-ui-fg-base">
                  {address.company}
                </span>
              )}
              <span className="text-base-regular mt-2">
                {address.address_1}
                {address.address_2 && <span>, {address.address_2}</span>}
              </span>
              <span className="text-base-regular">
                {address.postal_code}, {address.city}
              </span>
              <span className="text-base-regular">
                {address.province && `${address.province}, `}
                {address.country_code?.toUpperCase()}
              </span>
            </div>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

export default AddressSelect
