"use client"

import { Dialog, DialogContent, DialogTitle } from "@hanzo/ui"
import { ArrowRightMini, XMark } from "@hanzo/commerce-icons"
import { Text, clx, useToggleState } from "@hanzo/commerce-ui"
import { useState } from "react"

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
  const [open, setOpen] = useState(false)
  const close = () => setOpen(false)
  const countryToggleState = useToggleState()
  const languageToggleState = useToggleState()

  return (
    <div className="h-full">
      <div className="flex items-center h-full">
        {/* A full-height panel with its own hand-drawn backdrop, its own click
            handler to dismiss, and no escape key or focus trap is a dialog that
            was missing three of a dialog's four parts. DialogContent brings all
            of them; the only thing said here is that this one is anchored left
            rather than centred. */}
        <button
          data-testid="nav-menu-button"
          onClick={() => setOpen(true)}
          className="relative h-full flex items-center transition-all ease-out duration-200 focus:outline-none hover:text-ui-fg-base"
        >
          Menu
        </button>

        <Dialog modal open={open} onOpenChange={setOpen}>
          <DialogContent
            data-testid="nav-menu-popup"
            showCloseButton={false}
            p={0}
            style={{
              position: "fixed",
              inset: "0.5rem auto 0.5rem 0.5rem",
              transform: "none",
              width: "min(100%, 28rem)",
              maxWidth: "none",
              height: "calc(100vh - 1rem)",
            }}
          >
            <DialogTitle style={{ position: "absolute", left: -9999 }}>
              Menu
            </DialogTitle>
            <div className="grid h-full bg-[rgba(3,7,18,0.5)] rounded-rounded p-6 text-sm text-ui-fg-on-color backdrop-blur-2xl" style={{ gridTemplateRows: "auto 1fr auto" }}>
              <div className="flex justify-end" id="xmark">
                <button data-testid="close-menu-button" onClick={close}>
                  <XMark />
                </button>
              </div>
              <ul className="grid gap-6 justify-items-start content-start">
                {Object.entries(SideMenuItems).map(([name, href]) => (
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
                ))}
              </ul>
              <div className="grid gap-y-6">
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
          </DialogContent>
        </Dialog>
      </div>
    </div>
  )
}

export default SideMenu
