'use client'

import { useState, useEffect, useRef, useCallback } from 'react'
import { usePathname } from 'next/navigation'

const GATEWAY_URL = process.env.NEXT_PUBLIC_GATEWAY_URL || 'https://api.hanzo.ai/v1'
const GATEWAY_KEY = process.env.NEXT_PUBLIC_GATEWAY_PUBLIC_KEY || 'hz_widget_public'

const models = [
  { id: 'llama-3.3-70b', name: 'Llama 70B', tag: '70B' },
  { id: 'claude-haiku-4-5', name: 'Haiku 4.5', tag: 'Claude' },
  { id: 'gpt-4o-mini', name: 'GPT-4o Mini', tag: 'GPT' },
  { id: 'deepseek-r1-distill-70b', name: 'DeepSeek R1', tag: '70B' },
]

interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
}

export function ChatWidget() {
  const pathname = usePathname()
  const [isOpen, setIsOpen] = useState(false)
  const [input, setInput] = useState('')
  const [messages, setMessages] = useState<Message[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [selectedModel, setSelectedModel] = useState(models[0])
  const [showModels, setShowModels] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const modelRef = useRef<HTMLDivElement>(null)

  // Close model dropdown on outside click
  useEffect(() => {
    if (!showModels) return
    const handler = (e: MouseEvent) => {
      if (modelRef.current && !modelRef.current.contains(e.target as Node)) setShowModels(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [showModels])

  // Auto-scroll messages
  useEffect(() => { messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [messages])

  // Focus input when opened
  useEffect(() => { if (isOpen) inputRef.current?.focus() }, [isOpen])

  // Welcome message
  useEffect(() => {
    if (isOpen && messages.length === 0) {
      setMessages([{
        id: 'welcome',
        role: 'assistant',
        content: `Hi! I'm Zen AI, here to help with Hanzo Commerce. How can I assist you?`,
      }])
    }
  }, [isOpen])

  const handleSend = useCallback(async () => {
    if (!input.trim() || isLoading) return

    const userMsg: Message = { id: Date.now().toString(), role: 'user', content: input.trim() }
    setMessages(prev => [...prev, userMsg])
    setInput('')
    setIsLoading(true)

    try {
      const resp = await fetch(`${GATEWAY_URL}/chat/completions`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${GATEWAY_KEY}`,
        },
        body: JSON.stringify({
          model: selectedModel.id,
          messages: [
            {
              role: 'system',
              content: `You are Zen AI, Hanzo's commerce assistant. You're powered by ${selectedModel.name}. Current page: ${pathname}. Be helpful, concise, and knowledgeable about Hanzo Commerce — products, orders, inventory, API, pricing, and analytics. Direct users to /login for the admin dashboard or docs.hanzo.ai for documentation.`,
            },
            ...messages.slice(-10).map(m => ({ role: m.role, content: m.content })),
            { role: 'user', content: input.trim() },
          ],
          max_tokens: 800,
          temperature: 0.7,
        }),
      })

      if (resp.ok) {
        const data = await resp.json()
        setMessages(prev => [...prev, {
          id: (Date.now() + 1).toString(),
          role: 'assistant',
          content: data.choices[0].message.content,
        }])
      } else {
        setMessages(prev => [...prev, {
          id: (Date.now() + 1).toString(),
          role: 'assistant',
          content: 'I\'m having trouble connecting right now. Try docs.hanzo.ai for documentation or /login for the dashboard.',
        }])
      }
    } catch {
      setMessages(prev => [...prev, {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: 'Connection issue. Please try again or visit docs.hanzo.ai.',
      }])
    } finally {
      setIsLoading(false)
    }
  }, [input, isLoading, pathname, messages, selectedModel])

  return (
    <>
      {/* Floating button */}
      {!isOpen && (
        <button
          onClick={() => setIsOpen(true)}
          className="fixed bottom-6 right-6 z-50 flex h-14 w-14 items-center justify-center rounded-full border border-white/10 bg-[#111] shadow-lg transition-transform hover:scale-105 active:scale-95"
        >
          <svg viewBox="0 0 67 67" className="h-7 w-7 text-white" xmlns="http://www.w3.org/2000/svg">
            <path d="M22.21 67V44.6369H0V67H22.21Z" fill="currentColor"/>
            <path d="M0 44.6369L22.21 46.8285V44.6369H0Z" fill="currentColor" opacity="0.7"/>
            <path d="M66.7038 22.3184H22.2534L0.0878906 44.6367H44.4634L66.7038 22.3184Z" fill="currentColor"/>
            <path d="M22.21 0H0V22.3184H22.21V0Z" fill="currentColor"/>
            <path d="M66.7198 0H44.5098V22.3184H66.7198V0Z" fill="currentColor"/>
            <path d="M66.6753 22.3185L44.5098 20.0822V22.3185H66.6753Z" fill="currentColor" opacity="0.7"/>
            <path d="M66.7198 67V44.6369H44.5098V67H66.7198Z" fill="currentColor"/>
          </svg>
          <span className="absolute inset-0 rounded-full border border-white/5 animate-ping opacity-20" />
        </button>
      )}

      {/* Chat panel */}
      {isOpen && (
        <div className="fixed bottom-6 right-6 z-50 flex h-[520px] max-h-[80vh] w-[380px] max-w-[calc(100vw-48px)] flex-col overflow-hidden rounded-2xl border border-white/10 bg-[#0c0c0c] shadow-2xl">
          {/* Header */}
          <div className="flex items-center justify-between border-b border-white/[0.06] px-4 py-3">
            <div className="flex items-center gap-3">
              <svg viewBox="0 0 67 67" className="h-6 w-6 text-white" xmlns="http://www.w3.org/2000/svg">
                <path d="M22.21 67V44.6369H0V67H22.21Z" fill="currentColor"/>
                <path d="M0 44.6369L22.21 46.8285V44.6369H0Z" fill="currentColor" opacity="0.7"/>
                <path d="M66.7038 22.3184H22.2534L0.0878906 44.6367H44.4634L66.7038 22.3184Z" fill="currentColor"/>
                <path d="M22.21 0H0V22.3184H22.21V0Z" fill="currentColor"/>
                <path d="M66.7198 0H44.5098V22.3184H66.7198V0Z" fill="currentColor"/>
                <path d="M66.6753 22.3185L44.5098 20.0822V22.3185H66.6753Z" fill="currentColor" opacity="0.7"/>
                <path d="M66.7198 67V44.6369H44.5098V67H66.7198Z" fill="currentColor"/>
              </svg>
              {/* Model selector */}
              <div className="relative" ref={modelRef}>
                <button
                  onClick={() => setShowModels(!showModels)}
                  className="flex items-center gap-1.5 rounded-md px-2 py-1 text-sm font-medium text-white hover:bg-white/5"
                >
                  {selectedModel.name}
                  <span className="rounded bg-white/10 px-1 py-0.5 font-mono text-[10px] text-gray-400">{selectedModel.tag}</span>
                  <svg className={`h-3.5 w-3.5 text-gray-500 transition-transform ${showModels ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
                  </svg>
                </button>
                {showModels && (
                  <div className="absolute left-0 top-full mt-1 w-52 overflow-hidden rounded-lg border border-white/10 bg-[#111] shadow-xl">
                    {models.map(m => (
                      <button
                        key={m.id}
                        onClick={() => { setSelectedModel(m); setShowModels(false) }}
                        className={`flex w-full items-center justify-between px-3 py-2 text-left text-sm hover:bg-white/5 ${selectedModel.id === m.id ? 'bg-white/5' : ''}`}
                      >
                        <span className="text-white">{m.name} <span className="ml-1 font-mono text-[10px] text-gray-500">{m.tag}</span></span>
                        {selectedModel.id === m.id && (
                          <svg className="h-4 w-4 text-white/70" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                            <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                          </svg>
                        )}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            </div>
            <button onClick={() => setIsOpen(false)} className="rounded-md p-1.5 text-gray-500 hover:bg-white/5 hover:text-white">
              <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          {/* Messages */}
          <div className="flex-1 overflow-y-auto p-4 space-y-3">
            {messages.map(msg => (
              <div key={msg.id} className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}>
                <div className={`max-w-[85%] rounded-2xl px-3 py-2 text-sm ${
                  msg.role === 'user'
                    ? 'rounded-br-md bg-white text-black'
                    : 'rounded-bl-md bg-white/[0.06] text-gray-300'
                }`}>
                  {msg.content}
                </div>
              </div>
            ))}
            {isLoading && (
              <div className="flex justify-start">
                <div className="rounded-2xl rounded-bl-md bg-white/[0.06] px-4 py-2">
                  <div className="flex gap-1">
                    <span className="h-2 w-2 animate-bounce rounded-full bg-gray-500" style={{ animationDelay: '0ms' }} />
                    <span className="h-2 w-2 animate-bounce rounded-full bg-gray-500" style={{ animationDelay: '150ms' }} />
                    <span className="h-2 w-2 animate-bounce rounded-full bg-gray-500" style={{ animationDelay: '300ms' }} />
                  </div>
                </div>
              </div>
            )}
            <div ref={messagesEndRef} />
          </div>

          {/* Input */}
          <div className="border-t border-white/[0.06] p-3">
            <div className="relative">
              <input
                ref={inputRef}
                type="text"
                value={input}
                onChange={e => setInput(e.target.value)}
                onKeyDown={e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend() } }}
                placeholder="Ask anything..."
                className="w-full rounded-full border border-white/10 bg-white/[0.04] px-4 py-2.5 pr-12 text-sm text-white placeholder-gray-500 focus:border-white/20 focus:outline-none"
              />
              <button
                onClick={handleSend}
                disabled={!input.trim() || isLoading}
                className="absolute right-1.5 top-1/2 flex h-8 w-8 -translate-y-1/2 items-center justify-center rounded-full transition-all disabled:opacity-30"
                style={{ backgroundColor: input.trim() ? '#fff' : 'transparent' }}
              >
                <svg className={`h-4 w-4 ${input.trim() ? 'text-black' : 'text-gray-500'}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 12L3.269 3.126A59.768 59.768 0 0121.485 12 59.77 59.77 0 013.27 20.876L5.999 12zm0 0h7.5" />
                </svg>
              </button>
            </div>
            <p className="mt-1.5 text-center text-[10px] text-gray-600">
              Press Enter to send
            </p>
          </div>
        </div>
      )}
    </>
  )
}
