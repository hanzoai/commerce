import { describe, expect, it, vi } from 'vitest'
import { dispatch, parseAssistant, type AppHost } from './app-commands'

function host(authed = true): AppHost {
  return {
    isAuthed: () => authed,
    currentSection: () => 'products',
    navigate: vi.fn(),
    filter: vi.fn(),
    summarize: vi.fn(async () => 'summary'),
    createProduct: vi.fn(async input => ({ id: 'product_1', name: input.name })),
  }
}

describe('commerce assistant commands', () => {
  it('creates a product through the single host action', async () => {
    const app = host()
    const result = await dispatch([
      { type: 'create_product', name: 'Coffee', sku: 'COFFEE-1', description: 'Dark roast' },
    ], app)

    expect(app.createProduct).toHaveBeenCalledWith({
      name: 'Coffee',
      sku: 'COFFEE-1',
      description: 'Dark roast',
    })
    expect(result).toEqual([{ ok: true, message: 'Created draft product "Coffee"' }])
  })

  it('refuses actions without authentication', async () => {
    const app = host(false)
    const result = await dispatch([{ type: 'create_product', name: 'Coffee', sku: 'COFFEE-1' }], app)

    expect(app.createProduct).not.toHaveBeenCalled()
    expect(result[0].ok).toBe(false)
  })

  it('parses fenced command replies without executing them', () => {
    expect(parseAssistant('```json\n{"reply":"Ready","actions":[{"type":"navigate","section":"products"}]}\n```'))
      .toEqual({ reply: 'Ready', actions: [{ type: 'navigate', section: 'products' }] })
  })
})
