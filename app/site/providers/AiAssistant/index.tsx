"use client"

import { createContext, useContext, useRef, useState, type ReactNode } from "react"

type AiAssistantContextType = {
  chatOpened: boolean
  setChatOpened: (v: boolean) => void
  inputRef: React.RefObject<HTMLTextAreaElement | null>
  contentRef: React.RefObject<HTMLDivElement | null>
  loading: boolean
  isCaptchaLoaded: boolean
  submitQuery: (q: string) => void
  deepThinkingEnabled: boolean
  toggleDeepThinking: () => void
  suggestions: string[]
  hideAiToolsMessage: boolean
}

const AiAssistantContext = createContext<AiAssistantContextType>({
  chatOpened: false,
  setChatOpened: () => {},
  inputRef: { current: null },
  contentRef: { current: null },
  loading: false,
  isCaptchaLoaded: false,
  submitQuery: () => {},
  deepThinkingEnabled: false,
  toggleDeepThinking: () => {},
  suggestions: [],
  hideAiToolsMessage: true,
})

export function useAiAssistant() {
  return useContext(AiAssistantContext)
}

export function AiAssistantProvider({ children }: { children: ReactNode }) {
  const [chatOpened, setChatOpened] = useState(false)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const contentRef = useRef<HTMLDivElement>(null)

  return (
    <AiAssistantContext.Provider
      value={{
        chatOpened,
        setChatOpened,
        inputRef,
        contentRef,
        loading: false,
        isCaptchaLoaded: false,
        submitQuery: () => {},
        deepThinkingEnabled: false,
        toggleDeepThinking: () => {},
        suggestions: [],
        hideAiToolsMessage: true,
      }}
    >
      {children}
    </AiAssistantContext.Provider>
  )
}

export type AiAssistantThreadItem = {
  type: "question" | "answer"
  content: string
}
