import { Label, Switch } from "@hanzo/commerce-ui"

export default function SwitchDemo() {
  return (
    <div className="flex items-center gap-x-2">
      <Switch id="manage-inventory" />
      <Label htmlFor="manage-inventory">Manage Inventory</Label>
    </div>
  )
}
