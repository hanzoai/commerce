import { getLocale } from "@lib/data/locale-actions"

export async function getLocaleHeader() {
  const locale = await getLocale()
  return {
    "x-commerce-locale": locale,
  } as const
}
