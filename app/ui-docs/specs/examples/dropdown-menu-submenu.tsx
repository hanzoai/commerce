import { DropdownMenu, IconButton } from "@hanzo/commerce-ui"
import { BarsArrowDown } from "@hanzo/commerce-icons"

export default function DropdownMenuSubmenu() {
  return (
    <DropdownMenu>
      <DropdownMenu.Trigger asChild>
        <IconButton>
          <BarsArrowDown />
        </IconButton>
      </DropdownMenu.Trigger>
      <DropdownMenu.Content>
        <DropdownMenu.Item>Edit</DropdownMenu.Item>
        <DropdownMenu.SubMenu>
          <DropdownMenu.SubMenuTrigger>
            More Actions
          </DropdownMenu.SubMenuTrigger>
          <DropdownMenu.SubMenuContent>
            <DropdownMenu.Item>Duplicate</DropdownMenu.Item>
            <DropdownMenu.Item>Archive</DropdownMenu.Item>
          </DropdownMenu.SubMenuContent>
        </DropdownMenu.SubMenu>
        <DropdownMenu.Item>Delete</DropdownMenu.Item>
      </DropdownMenu.Content>
    </DropdownMenu>
  )
}
