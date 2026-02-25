import { Label, Switch } from "@hanzo/commerce-ui"

export default function SwitchDisabled() {
  return (
    <div className="flex items-center gap-x-2">
      <Switch id="manage-inventory-disabled" disabled={true} />
      <Label htmlFor="manage-inventory-disabled">Manage Inventory</Label>
    </div>
  )
}
