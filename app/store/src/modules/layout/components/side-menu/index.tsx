"use client"

import { ArrowRightMini, XMark } from "@hanzo/commerce-icons"
import { Text, clx, useToggleState } from "@hanzo/commerce-ui"
import { useEffect } from "react"

import LocalizedClientLink from "@modules/common/components/localized-client-link"
import CountrySelect from "../country-select"
import LanguageSelect from "../language-select"
import { HttpTypes } from "@hanzo/commerce-types"
import { Locale } from "@lib/data/locales"

const SideMenuItems = {
  Home: "/",
  Store: "/store",
  Account: "/account",
  Cart: "/cart",
}

type SideMenuProps = {
  regions: HttpTypes.StoreRegion[] | null
  locales: Locale[] | null
  currentLocale: string | null
}

const SideMenu = ({ regions, locales, currentLocale }: SideMenuProps) => {
  const countryToggleState = useToggleState()
  const languageToggleState = useToggleState()
  const { state: open, open: openMenu, close } = useToggleState()

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") close()
    }
    document.addEventListener("keydown", onKey)
    return () => document.removeEventListener("keydown", onKey)
  }, [open, close])

  return (
    <div className="h-full">
      <div className="flex items-center h-full">
        <div className="h-full flex">
          <div className="relative flex h-full">
            <button
              data-testid="nav-menu-button"
              aria-expanded={open}
              onClick={openMenu}
              className="relative h-full flex items-center transition-all ease-out duration-200 focus:outline-none hover:text-ui-fg-base"
            >
              Menu
            </button>
          </div>

          {open && (
            <div
              className="fixed inset-0 z-[50] bg-black/0 pointer-events-auto"
              onClick={close}
              data-testid="side-menu-backdrop"
            />
          )}

          {open && (
            <div className="flex flex-col absolute w-full pr-4 sm:pr-0 sm:w-1/3 2xl:w-1/4 sm:min-w-min h-[calc(100vh-1rem)] z-[51] inset-x-0 text-sm text-ui-fg-on-color m-2 backdrop-blur-2xl">
              <div
                data-testid="nav-menu-popup"
                className="flex flex-col h-full bg-[rgba(3,7,18,0.5)] rounded-rounded justify-between p-6"
              >
                <div className="flex justify-end" id="xmark">
                  <button data-testid="close-menu-button" onClick={close}>
                    <XMark />
                  </button>
                </div>
                <ul className="flex flex-col gap-6 items-start justify-start">
                  {Object.entries(SideMenuItems).map(([name, href]) => {
                    return (
                      <li key={name}>
                        <LocalizedClientLink
                          href={href}
                          className="text-3xl leading-10 hover:text-ui-fg-disabled"
                          onClick={close}
                          data-testid={`${name.toLowerCase()}-link`}
                        >
                          {name}
                        </LocalizedClientLink>
                      </li>
                    )
                  })}
                </ul>
                <div className="flex flex-col gap-y-6">
                  {!!locales?.length && (
                    <div
                      className="flex justify-between"
                      onMouseEnter={languageToggleState.open}
                      onMouseLeave={languageToggleState.close}
                    >
                      <LanguageSelect
                        toggleState={languageToggleState}
                        locales={locales}
                        currentLocale={currentLocale}
                      />
                      <ArrowRightMini
                        className={clx(
                          "transition-transform duration-150",
                          languageToggleState.state ? "-rotate-90" : ""
                        )}
                      />
                    </div>
                  )}
                  <div
                    className="flex justify-between"
                    onMouseEnter={countryToggleState.open}
                    onMouseLeave={countryToggleState.close}
                  >
                    {regions && (
                      <CountrySelect
                        toggleState={countryToggleState}
                        regions={regions}
                      />
                    )}
                    <ArrowRightMini
                      className={clx(
                        "transition-transform duration-150",
                        countryToggleState.state ? "-rotate-90" : ""
                      )}
                    />
                  </div>
                  <Text className="flex justify-between txt-compact-small">
                    © 2026 Hanzo AI. All rights reserved.
                  </Text>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export default SideMenu
