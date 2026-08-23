"use client"

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@hanzo/ui"
import { useEffect, useMemo, useState } from "react"
import ReactCountryFlag from "react-country-flag"

import { StateType } from "@lib/hooks/use-toggle-state"
import { useParams, usePathname } from "next/navigation"
import { updateRegion } from "@lib/data/cart"
import { HttpTypes } from "@hanzo/commerce-types"

type CountryOption = {
  country: string
  region: string
  label: string
}

type CountrySelectProps = {
  toggleState: StateType
  regions: HttpTypes.StoreRegion[]
}

/**
 * The country picker, on the kit's Select.
 *
 * It was a Headless UI Listbox whose options were `static` and whose visibility
 * came from a `Transition show={state}` driven by the side menu's hover — so
 * the Listbox's own open state was never consulted and the panel had to be
 * placed by hand at `-bottom-[calc(100%-36px)]`. Select takes that same state
 * as `open`, and places its own list.
 *
 * The option VALUE is now the country code rather than the option object: a
 * select's value is a string, and the object is one lookup away.
 */
const CountrySelect = ({ toggleState, regions }: CountrySelectProps) => {
  const [current, setCurrent] = useState<CountryOption | undefined>(undefined)

  const { countryCode } = useParams()
  const currentPath = usePathname().split(`/${countryCode}`)[1]

  const { state, close } = toggleState

  const options = useMemo(
    () =>
      regions
        ?.flatMap((r) =>
          (r.countries ?? []).map((c) => ({
            country: c.iso_2 ?? "",
            region: r.id,
            label: c.display_name ?? "",
          }))
        )
        .sort((a, b) => a.label.localeCompare(b.label)),
    [regions]
  )

  useEffect(() => {
    if (countryCode) {
      setCurrent(options?.find((o) => o.country === countryCode))
    }
  }, [options, countryCode])

  const handleChange = (country: string) => {
    updateRegion(country, currentPath)
    close()
  }

  return (
    <Select
      value={current?.country}
      onValueChange={handleChange}
      open={state}
      onOpenChange={(open) => !open && close()}
    >
      <SelectTrigger data-testid="country-select">
        <SelectValue placeholder="Shipping to:">
          {current ? (
            <span className="txt-compact-small flex items-center gap-x-2">
              {/* @ts-ignore */}
              <ReactCountryFlag
                svg
                style={{ width: "16px", height: "16px" }}
                countryCode={current.country}
              />
              {current.label}
            </span>
          ) : (
            "Shipping to:"
          )}
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        {options?.map((o, index) => (
          <SelectItem key={o.country} index={index} value={o.country}>
            <span className="flex items-center gap-x-2">
              {/* @ts-ignore */}
              <ReactCountryFlag
                svg
                style={{ width: "16px", height: "16px" }}
                countryCode={o.country}
              />
              {o.label}
            </span>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

export default CountrySelect
