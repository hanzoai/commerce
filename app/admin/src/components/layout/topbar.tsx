import { BarsThree, MagnifyingGlass } from '@hanzo/commerce-icons'
import { Kbd } from '@hanzo/commerce-ui'
import { HanzoMark } from '@/components/hanzo-mark'
import { StoreMenu } from './store-menu'

export function Topbar({
  onOpen,
  onSearch,
}: {
  onOpen: () => void
  onSearch: () => void
}) {
  return (
    <header className="sticky top-0 z-40 flex h-14 items-center gap-3 border-b border-ui-border-base bg-ui-bg-base/80 px-4 backdrop-blur sm:px-6">
      <button
        type="button"
        aria-label="Open navigation"
        onClick={onOpen}
        className="rounded-md p-2 text-ui-fg-muted hover:bg-ui-bg-component hover:text-ui-fg-base lg:hidden"
      >
        <BarsThree className="h-5 w-5" />
      </button>
      <HanzoMark className="h-6 w-6 shrink-0 text-ui-fg-base lg:hidden" />

      <button
        type="button"
        onClick={onSearch}
        aria-label="Search (⌘K)"
        aria-keyshortcuts="Meta+K Control+K"
        className="hidden h-9 items-center gap-2 rounded-md border border-ui-border-base bg-ui-bg-field px-3 text-sm text-ui-fg-muted transition-colors hover:bg-ui-bg-field-hover sm:flex"
      >
        <MagnifyingGlass className="h-4 w-4" />
        <span className="hidden md:inline">Search…</span>
        <Kbd className="ml-2 hidden md:inline-flex">⌘K</Kbd>
      </button>
      <button
        type="button"
        onClick={onSearch}
        aria-label="Search"
        className="rounded-md p-2 text-ui-fg-muted hover:bg-ui-bg-component hover:text-ui-fg-base sm:hidden"
      >
        <MagnifyingGlass className="h-5 w-5" />
      </button>

      <div className="ml-auto min-w-0">
        <StoreMenu />
      </div>
    </header>
  )
}
