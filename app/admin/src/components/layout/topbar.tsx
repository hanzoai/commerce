import { BarsThree } from '@hanzo/commerce-icons'
import { HanzoMark } from '@/components/hanzo-mark'
import { StoreMenu } from './store-menu'

export function Topbar({ onOpen }: { onOpen: () => void }) {
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
      <div className="ml-auto min-w-0 lg:ml-0">
        <StoreMenu />
      </div>
    </header>
  )
}
