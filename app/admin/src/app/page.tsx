'use client'

// The admin has one entry: the merchant overview. A static export cannot redirect
// server-side, so `/` bounces on mount (the auth gate in (dashboard) takes it from
// there).
import { useEffect } from 'react'
import { useRouter } from 'next/navigation'

export default function IndexPage() {
  const router = useRouter()
  useEffect(() => router.replace('/overview'), [router])
  return null
}
