import { HttpTypes } from "@hanzo/commerce-types"
import { Badge } from "@hanzo/commerce-ui"
import { createColumnHelper } from "@tanstack/react-table"
import { useMemo } from "react"

const columnHelper = createColumnHelper<HttpTypes.AdminReturnReason>()

export const useReturnReasonTableColumns = () => {
  return useMemo(
    () => [
      columnHelper.accessor("value", {
        cell: ({ getValue }) => <Badge size="2xsmall">{getValue()}</Badge>,
      }),
      columnHelper.accessor("label", {
        cell: ({ row }) => {
          const { label, description } = row.original
          return (
            <div className=" py-4">
              <div className="flex h-full w-full flex-col justify-center">
                <span className="truncate font-medium">{label}</span>
                <span className="truncate">
                  {description ? description : "-"}
                </span>
              </div>
            </div>
          )
        },
      }),
    ],
    []
  )
}
