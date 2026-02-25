import { QueryClient } from "@tanstack/react-query"

export const HANZO_COMMERCE_BACKEND_URL =
  process.env.__BACKEND_URL__ ?? "https://api.commerce.hanzo.ai"

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      staleTime: 90000,
      retry: 1,
    },
  },
})
