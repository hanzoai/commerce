import { CurrencyInput } from "@hanzo/commerce-ui"

export default function CurrencyInputSmall() {
  return (
    <div className="max-w-[250px]">
      <CurrencyInput size="small" symbol="$" code="usd" />
    </div>
  )
}
