import HanzoCommerce from "@hanzo/commerce-sdk"

export const backendUrl =
  process.env.__BACKEND_URL__ ?? "https://api.commerce.hanzo.ai"
const authType = (process.env.__AUTH_TYPE__ as "session" | "jwt") ?? "session"
const jwtTokenStorageKey = process.env.__JWT_TOKEN_STORAGE_KEY__ || undefined

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
