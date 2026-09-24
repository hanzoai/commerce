"use client"

import { createContext, useContext, type ReactNode } from "react"

export type ExtraData = {
  section?: string
  [key: string]: unknown
}

type AnalyticsContextType = {
  track: (event: string, data?: ExtraData) => void
}

const AnalyticsContext = createContext<AnalyticsContextType>({
  track: () => {},
})

export function useAnalytics() {
  return useContext(AnalyticsContext)
}

export function AnalyticsProvider({ children }: { children: ReactNode }) {
  return (
    <AnalyticsContext.Provider value={{ track: () => {} }}>
      {children}
    </AnalyticsContext.Provider>
  )
}
