"use client"

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@hanzo/ui"
import { useEffect, useMemo, useState, useTransition } from "react"
import { useRouter } from "next/navigation"
import ReactCountryFlag from "react-country-flag"

import { StateType } from "@lib/hooks/use-toggle-state"
import { updateLocale } from "@lib/data/locale-actions"
import { Locale } from "@lib/data/locales"

type LanguageOption = {
  code: string
  name: string
  localizedName: string
  countryCode: string
}

const getCountryCodeFromLocale = (localeCode: string): string => {
  try {
    const locale = new Intl.Locale(localeCode)
    if (locale.region) {
      return locale.region.toUpperCase()
    }
    const maximized = locale.maximize()
    return maximized.region?.toUpperCase() ?? localeCode.toUpperCase()
  } catch {
    const parts = localeCode.split(/[-_]/)
    return parts.length > 1 ? parts[1].toUpperCase() : parts[0].toUpperCase()
  }
}

type LanguageSelectProps = {
  toggleState: StateType
  locales: Locale[]
  currentLocale: string | null
}

/**
 * Gets the localized display name for a language code using Intl API.
 * Falls back to the provided name if Intl is unavailable.
 */
const getLocalizedLanguageName = (
  code: string,
  fallbackName: string,
  displayLocale: string = "en-US"
): string => {
  try {
    const displayNames = new Intl.DisplayNames([displayLocale], {
      type: "language",
    })
    return displayNames.of(code) ?? fallbackName
  } catch {
    return fallbackName
  }
}

/** A select's value is a string and "" is not one — an empty value reads as
 *  "nothing chosen", which is a different fact from "the default locale". */
const DEFAULT_CODE = "default"

const DEFAULT_OPTION: LanguageOption = {
  code: "",
  name: "Default",
  localizedName: "Default",
  countryCode: "",
}

const LanguageSelect = ({
  toggleState,
  locales,
  currentLocale,
}: LanguageSelectProps) => {
  const [current, setCurrent] = useState<LanguageOption | undefined>(undefined)
  const [isPending, startTransition] = useTransition()
  const router = useRouter()

  const { state, close } = toggleState

  const options = useMemo(() => {
    const localeOptions = locales.map((locale) => ({
      code: locale.code,
      name: locale.name,
      localizedName: getLocalizedLanguageName(
        locale.code,
        locale.name,
        currentLocale ?? "en-US"
      ),
      countryCode: getCountryCodeFromLocale(locale.code),
    }))
    return [DEFAULT_OPTION, ...localeOptions]
  }, [locales, currentLocale])

  useEffect(() => {
    if (currentLocale) {
      const option = options.find(
        (o) => o.code.toLowerCase() === currentLocale.toLowerCase()
      )
      setCurrent(option ?? DEFAULT_OPTION)
    } else {
      setCurrent(DEFAULT_OPTION)
    }
  }, [options, currentLocale])

  const handleChange = (code: string) => {
    startTransition(async () => {
      await updateLocale(code === DEFAULT_CODE ? "" : code)
      close()
      router.refresh()
    })
  }

  return (
    <div>
      <Select
        value={current?.code || DEFAULT_CODE}
        onValueChange={handleChange}
        open={state}
        onOpenChange={(open) => !open && close()}
      >
        <SelectTrigger data-testid="language-select" disabled={isPending}>
          <SelectValue placeholder="Language:">
            <span className="txt-compact-small flex items-center gap-x-2">
              {current?.countryCode && (
                /* @ts-ignore */
                <ReactCountryFlag
                  svg
                  style={{ width: "16px", height: "16px" }}
                  countryCode={current.countryCode}
                />
              )}
              {isPending ? "..." : current?.localizedName ?? "Language:"}
            </span>
          </SelectValue>
        </SelectTrigger>
        <SelectContent>
          {options.map((o, index) => (
            <SelectItem
              key={o.code || DEFAULT_CODE}
              index={index}
              value={o.code || DEFAULT_CODE}
            >
              <span className="flex items-center gap-x-2">
                {o.countryCode ? (
                  /* @ts-ignore */
                  <ReactCountryFlag
                    svg
                    style={{ width: "16px", height: "16px" }}
                    countryCode={o.countryCode}
                  />
                ) : (
                  <span style={{ width: "16px", height: "16px" }} />
                )}
                {o.localizedName}
              </span>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}

export default LanguageSelect
