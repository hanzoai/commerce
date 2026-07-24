import { describe, expect, it, vi } from 'vitest'
import { dispatch, parseAssistant, type AppHost, type ProductSpec } from './app-commands'

function host(authed = true): AppHost {
  return {
    isAuthed: () => authed,
    currentSection: () => 'products',
    navigate: vi.fn(),
    filter: vi.fn(),
    summarize: vi.fn(async () => 'summary'),
    createProduct: vi.fn(async input => ({ id: 'product_1', name: input.name })),
    createCollection: vi.fn(async input => ({ id: 'collection_1', title: input.title })),
    createStore: vi.fn(async input => ({ id: 'store_1', name: input.name })),
    generateCatalog: vi.fn(async (_theme: string, _count: number, specs: ProductSpec[]) => ({
      created: specs.map((s, i) => ({ id: `product_${i + 1}`, name: s.name })),
      failed: 0,
    })),
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
    expect(result).toEqual([
      { ok: true, message: 'Created draft product "Coffee"', href: '/products/product_1' },
    ])
  })

  it('generates a themed catalog through the batch host action', async () => {
    const app = host()
    const result = await dispatch([
      {
        type: 'generate_catalog',
        theme: 'artisan coffee',
        count: '2',
        products: [{ name: 'Cold Brew', priceUsd: 6 }, { name: 'Espresso', priceUsd: 4 }],
      },
    ], app)

    expect(app.generateCatalog).toHaveBeenCalledWith('artisan coffee', 2, [
      { name: 'Cold Brew', priceUsd: 6, description: undefined, sku: undefined, status: undefined },
      { name: 'Espresso', priceUsd: 4, description: undefined, sku: undefined, status: undefined },
    ])
    expect(result[0].ok).toBe(true)
    expect(result[0].href).toBe('/products')
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
