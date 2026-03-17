"use client"

import { useEffect, useState } from "react"
import { Suspense } from "react"
import { handleCallback } from "../../lib/iam-auth"

function CallbackHandler() {
  const [status, setStatus] = useState<"loading" | "error">("loading")
  const [error, setError] = useState("")

  useEffect(() => {
    handleCallback()
      .then((redirectUrl) => {
        window.location.href = redirectUrl
      })
      .catch((err) => {
        setStatus("error")
        setError(err instanceof Error ? err.message : "Token exchange failed")
      })
  }, [])

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
