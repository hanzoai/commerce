import HanzoCommerce from "@hanzo/commerce-sdk"

export const backendUrl =
  process.env.NEXT_PUBLIC_HANZO_COMMERCE_BACKEND_URL ?? "https://api.commerce.hanzo.ai"
const authType = (process.env.NEXT_PUBLIC_HANZO_COMMERCE_AUTH_TYPE as "session" | "jwt") ?? "session"
const jwtTokenStorageKey = process.env.NEXT_PUBLIC_HANZO_COMMERCE_JWT_TOKEN_STORAGE_KEY || undefined

export const sdk = new HanzoCommerce({
  baseUrl: backendUrl,
  auth: {
    type: authType,
    jwtTokenStorageKey,
  },
})

// useful when you want to call the BE from the console and try things out quickly
if (typeof window !== "undefined") {
  ;(window as any).__sdk = sdk
}
