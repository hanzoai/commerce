"use client"

import { usePathname } from "next/navigation"

const EditButton = () => {
  const pathname = usePathname()

  const filePath = `app/site/app${pathname.replace(/\/$/, "")}/page.mdx`
  const editUrl = `https://github.com/hanzoai/commerce/${filePath}`

  return (
    <a
      href={editUrl}
      target="_blank"
      rel="noopener noreferrer"
      className="text-xs text-gray-400 hover:text-gray-600"
    >
      Edit this page on GitHub
    </a>
  )
}

export default EditButton
