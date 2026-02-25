import { getLocaleHeader } from "@lib/util/get-locale-header"
import HanzoCommerce, { FetchArgs, FetchInput } from "@hanzo/commerce-sdk"

// Defaults to Hanzo Commerce API
let HANZO_COMMERCE_API_URL = "https://api.commerce.hanzo.ai"

if (process.env.HANZO_COMMERCE_API_URL) {
  HANZO_COMMERCE_API_URL = process.env.HANZO_COMMERCE_API_URL
}

export const sdk = new HanzoCommerce({
  baseUrl: HANZO_COMMERCE_API_URL,
  debug: process.env.NODE_ENV === "development",
  publishableKey: process.env.NEXT_PUBLIC_HANZO_COMMERCE_KEY,
})

const originalFetch = sdk.client.fetch.bind(sdk.client)

sdk.client.fetch = async <T>(
  input: FetchInput,
  init?: FetchArgs
): Promise<T> => {
  const headers = init?.headers ?? {}
  let localeHeader: Record<string, string | null> | undefined
  try {
    localeHeader = await getLocaleHeader()
    headers["x-commerce-locale"] ??= localeHeader["x-commerce-locale"]
  } catch {}

  const newHeaders = {
    ...localeHeader,
    ...headers,
  }
  init = {
    ...init,
    headers: newHeaders,
  }
  return originalFetch(input, init)
}
