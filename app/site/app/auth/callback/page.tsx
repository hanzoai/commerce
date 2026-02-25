"use client"

import { useEffect, useState } from "react"
import { useSearchParams } from "next/navigation"
import { Suspense } from "react"

function CallbackHandler() {
  const searchParams = useSearchParams()
  const [status, setStatus] = useState<"loading" | "error">("loading")
  const [error, setError] = useState("")

  useEffect(() => {
    const code = searchParams.get("code")
    const state = searchParams.get("state")
    const errorParam = searchParams.get("error")

    if (errorParam) {
      setStatus("error")
      setError(searchParams.get("error_description") || errorParam)
      return
    }

    if (!code) {
      setStatus("error")
      setError("No authorization code received.")
      return
    }

    exchangeToken(code, state)
  }, [searchParams])

  async function exchangeToken(code: string, state: string | null) {
    try {
      const response = await fetch(
        "https://hanzo.id/api/login/oauth/access_token",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            grant_type: "authorization_code",
            client_id: "hanzo-app-client-id",
            code,
            redirect_uri: "https://commerce.hanzo.ai/auth/callback",
          }),
        }
      )

      if (!response.ok) {
        const text = await response.text()
        throw new Error(text || `Token exchange failed (${response.status})`)
      }

      const data = await response.json()
      sessionStorage.setItem("hanzo_commerce_access_token", data.access_token)
      if (data.refresh_token) {
        sessionStorage.setItem(
          "hanzo_commerce_refresh_token",
          data.refresh_token
        )
      }
      if (data.id_token) {
        sessionStorage.setItem("hanzo_commerce_id_token", data.id_token)
      }

      window.location.href = state || "https://admin.commerce.hanzo.ai"
    } catch (err) {
      setStatus("error")
      setError(err instanceof Error ? err.message : "Token exchange failed")
    }
  }

  if (status === "error") {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-[#0a0a0a] px-6">
        <div className="mx-auto max-w-md rounded-2xl border border-red-500/20 bg-red-500/5 p-8 text-center">
          <h1 className="mb-2 text-xl font-semibold text-white">
            Authentication Error
          </h1>
          <p className="text-sm text-gray-400">{error}</p>
          <a
            href="/"
            className="mt-6 inline-block rounded-lg bg-brand px-4 py-2 text-sm font-medium text-white"
          >
            Back to Home
          </a>
        </div>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-[#0a0a0a]">
      <div className="h-8 w-8 animate-spin rounded-full border-2 border-brand border-t-transparent" />
      <p className="mt-4 text-sm text-gray-400">Signing you in...</p>
    </div>
  )
}

export default function AuthCallbackPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center bg-[#0a0a0a]">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-brand border-t-transparent" />
        </div>
      }
    >
      <CallbackHandler />
    </Suspense>
  )
}
