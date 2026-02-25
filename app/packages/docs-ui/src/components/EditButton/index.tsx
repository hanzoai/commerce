import React from "react"
import Link from "next/link"
import clsx from "clsx"
import { ArrowUpRightOnBox } from "@hanzo/commerce-icons"
import { EditDate } from "../EditDate"

type EditButtonProps = {
  filePath: string
  editDate?: string
}

export const EditButton = ({ filePath, editDate }: EditButtonProps) => {
  return (
    <div className="flex flex-wrap gap-docs_0.5 mt-docs_2 text-hanzo-fg-subtle">
      {editDate && <EditDate date={editDate} />}

      <Link
        href={`https://github.com/hanzoai/commerce/edit/develop${filePath}`}
        className={clsx(
          "flex w-fit gap-docs_0.25 items-center",
          "text-hanzo-fg-subtle hover:text-hanzo-fg-base",
          "text-compact-small-plus"
        )}
        data-testid="edit-button"
      >
        <span>Edit this page</span>
        <ArrowUpRightOnBox />
      </Link>
    </div>
  )
}
