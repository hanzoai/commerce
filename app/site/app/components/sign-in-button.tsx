"use client"

import { startLogin } from "../lib/iam-auth"

export function SignInButton({ className }: { className?: string }) {
  return (
    <button
      onClick={() => startLogin()}
      className={className}
    >
      Sign In
    </button>
  )
}
