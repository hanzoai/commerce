import { AccountMenu } from './account-menu'

export function Topbar() {
  return (
    <header className="sticky top-0 z-40 flex h-14 items-center justify-between border-b border-ui-border-base bg-ui-bg-base/80 px-6 backdrop-blur">
      <div />
      <AccountMenu />
    </header>
  )
}
