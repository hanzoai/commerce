import { CustomFieldModel } from "@hanzo/commerce-admin-shared"
import linkModule from "../../lib/extensions/links"

function appendLinkableFields(
  fields: string = "",
  linkable: (string | string[])[] = []
) {
  const linkableFields = linkable.flatMap((link) => {
    return typeof link === "string"
      ? [`+${link}.*`]
      : link.map((l) => `+${l}.*`)
  })

  return [fields, ...linkableFields].join(",")
}

export function getLinkedFields(model: CustomFieldModel, fields: string = "") {
  const links = linkModule.links[model]
  return appendLinkableFields(fields, links)
}
