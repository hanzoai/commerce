import { Input } from "@hanzo/commerce-ui"

export default function InputDisabled() {
  return (
    <div className="w-[250px]">
      <Input placeholder="Disabled" id="disabled-input" disabled />
    </div>
  )
}
