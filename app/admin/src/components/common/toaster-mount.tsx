'use client'

// Mounts the toast surface. The live app shell does not render a <Toaster>, so
// pages that raise success/error toasts include this once at their root to make
// those toasts visible. Sonner-based: a bare <Toaster /> with no props renders the
// queue that the global `toast()` writes to. Safe to keep even if the shell later
// adds its own — consolidate to one mount at that point.
import { Toaster } from '@hanzo/commerce-ui'

export function ToasterMount() {
  return <Toaster />
}
