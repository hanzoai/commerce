// ─────────────────────────────────────────────────────────────────────────────
// ONE control surface for the AI dock (decomplected, after world.hanzo.ai's
// app-commands): a PORT the app implements, a DATA table of ops (single source of
// truth), and ONE validated dispatcher. The backend/MCP tool contract is DERIVED
// from `commandManifest()` — never duplicated.
//
// The assistant is AGENTIC: alongside read/navigate ops it CREATES records the
// merchant asks for, through the same data-provider `createOne(...)` path the
// admin forms use (Bearer + X-Org-Id already attached).
//   read/move:  navigate · search · open · summarize
//   write:      create_product · create_collection · create_store · generate_catalog
// To add another op: append an AppCommand entry here + implement its method on
// AppHost. Nothing else changes — the prompt, manifest, validation and dispatch
// all read this table. Writes flow through dispatch's isAuthed() gate, so an
// unauthenticated caller can never mutate the store.
// ─────────────────────────────────────────────────────────────────────────────

export const SECTIONS = [
  'overview', 'models', 'products', 'orders', 'customers', 'collections', 'inventory', 'billing', 'settings',
] as const
export type Section = (typeof SECTIONS)[number]

/** Sections backed by a searchable list, and their data-provider kind. */
export const SECTION_TO_KIND: Record<string, string> = {
  products: 'product',
  orders: 'order',
  customers: 'c/user',
  collections: 'collection',
  inventory: 'stocklocation',
}
export const LIST_SECTIONS = Object.keys(SECTION_TO_KIND)

/** A single product the assistant asks the store to create. */
export interface ProductSpec {
  name: string
  description?: string
  priceUsd?: number
  sku?: string
  status?: string
}

/** What a write returns to the receipt: the new record's name + (optional) id to deep-link. */
export interface Created { id?: string; name: string }

// PORT — the narrow capability surface the assistant may drive. The dock implements it.
export interface AppHost {
  isAuthed(): boolean
  currentSection(): string
  navigate(section: string): void
  /** Drive a list section's search (navigates there + filters by query). */
  filter(section: string, query: string): void
  /** Fetch real data for a section and return a short human summary. */
  summarize(section: string): Promise<string>
  /** Create one product in the active store. */
  createProduct(input: ProductSpec): Promise<Created>
  /** Create one collection. */
  createCollection(input: { title: string; handle?: string }): Promise<{ id?: string; title: string }>
  /** Create one store. */
  createStore(input: { name: string; currency?: string }): Promise<Created>
  /** Create a batch of products (a themed catalog). Returns what landed. */
  generateCatalog(theme: string, count: number, specs: ProductSpec[]): Promise<{ created: Created[]; failed: number }>
}

export interface CommandResult { ok: boolean; message: string; href?: string }
export type CommandLogEntry = CommandResult
/** Per-action run context. `raw` carries the full action so a command can read
 *  non-scalar params (e.g. generate_catalog's `products[]`) the scalar validator skips. */
export interface RunCtx { label: (k: string) => string; raw: RawCommand }

interface JsonSchema {
  type: 'object'
  properties: Record<string, { type: string; enum?: readonly string[]; description?: string; items?: unknown }>
  required?: string[]
}

export interface AppCommand {
  name: string
  description: string
  params: JsonSchema
  run(host: AppHost, args: Record<string, string>, ctx: RunCtx): Promise<CommandResult> | CommandResult
}

/** The flat action the model emits: { type, ...params }. */
export interface RawCommand { type?: string; [k: string]: unknown }

export const COMMANDS: AppCommand[] = [
  {
    name: 'navigate',
    description: `Open an admin section. section is one of: ${SECTIONS.join(', ')}.`,
    params: { type: 'object', properties: { section: { type: 'string', enum: SECTIONS } }, required: ['section'] },
    run: (host, a) => {
      host.navigate(a.section)
      return { ok: true, message: `Opened ${a.section}` }
    },
  },
  {
    name: 'search',
    description: `Filter a list section by text (name / sku / email). section defaults to products; one of: ${LIST_SECTIONS.join(', ')}.`,
    params: {
      type: 'object',
      properties: { query: { type: 'string' }, section: { type: 'string', enum: LIST_SECTIONS } },
      required: ['query'],
    },
    run: (host, a) => {
      const section = a.section || 'products'
      host.filter(section, a.query)
      return { ok: true, message: `Searching ${section} for "${a.query}"` }
    },
  },
  {
    name: 'open',
    description: `Locate a specific record by id or slug inside its list. resource one of: ${LIST_SECTIONS.join(', ')}.`,
    params: {
      type: 'object',
      properties: { resource: { type: 'string', enum: LIST_SECTIONS }, id: { type: 'string' } },
      required: ['resource', 'id'],
    },
    run: (host, a) => {
      host.filter(a.resource, a.id)
      return { ok: true, message: `Locating ${a.resource} ${a.id}` }
    },
  },
  {
    name: 'create_product',
    description: 'Create a product in the active store. Only name is required; SKU is auto-derived when omitted. priceUsd is dollars (e.g. 19.99). status is one of draft, live, hidden (default draft). Use when the merchant asks to add/create a product.',
    params: {
      type: 'object',
      properties: {
        name: { type: 'string', description: 'Product name' },
        description: { type: 'string', description: 'Product description' },
        priceUsd: { type: 'string', description: 'Price in US dollars, e.g. 19.99' },
        sku: { type: 'string', description: 'Merchant SKU (auto-derived from name if omitted)' },
        status: { type: 'string', enum: ['draft', 'live', 'hidden'], description: 'Publish state' },
      },
      required: ['name'],
    },
    run: async (host, a) => {
      const p = await host.createProduct({
        name: a.name,
        description: a.description,
        priceUsd: a.priceUsd ? Number(a.priceUsd) : undefined,
        sku: a.sku,
        status: a.status,
      })
      return {
        ok: true,
        message: `Created ${a.status || 'draft'} product "${p.name}"`,
        href: p.id ? `/products/${p.id}` : '/products',
      }
    },
  },
  {
    name: 'create_collection',
    description: 'Create a collection to group products. Only title is required; handle is auto-derived when omitted. Use when the merchant asks to add/create a collection.',
    params: {
      type: 'object',
      properties: {
        title: { type: 'string', description: 'Collection title' },
        handle: { type: 'string', description: 'URL handle (auto-derived from title if omitted)' },
      },
      required: ['title'],
    },
    run: async (host, a) => {
      const c = await host.createCollection({ title: a.title, handle: a.handle })
      return { ok: true, message: `Created collection "${c.title}"`, href: c.id ? `/collections/${c.id}` : '/collections' }
    },
  },
  {
    name: 'create_store',
    description: 'Create a store. Only name is required; currency defaults to usd. Use when the merchant asks to add/create a store.',
    params: {
      type: 'object',
      properties: {
        name: { type: 'string', description: 'Store name' },
        currency: { type: 'string', description: 'ISO currency code, e.g. usd, eur' },
      },
      required: ['name'],
    },
    run: async (host, a) => {
      const s = await host.createStore({ name: a.name, currency: a.currency })
      return { ok: true, message: `Created store "${s.name}"`, href: '/settings' }
    },
  },
  {
    name: 'generate_catalog',
    description: 'Create a whole themed catalog at once. YOU invent `count` product specs that fit `theme` and pass them in `products` (each: { name, description?, priceUsd?, sku? }). Use when the merchant asks to generate/seed a catalog or many products.',
    params: {
      type: 'object',
      properties: {
        theme: { type: 'string', description: 'Catalog theme, e.g. "artisan coffee"' },
        count: { type: 'string', description: 'How many products to create (1-24)' },
        products: { type: 'array', description: 'The specs you invent: [{ name, description?, priceUsd?, sku? }]', items: { type: 'object' } },
      },
      required: ['theme', 'count'],
    },
    run: async (host, a, ctx) => {
      const count = Math.max(1, Math.min(24, Math.round(Number(a.count)) || 1))
      const specs = readSpecs(ctx.raw.products)
      const { created, failed } = await host.generateCatalog(a.theme, count, specs)
      const links = created.map((p) => (p.id ? `${p.name} (/products/${p.id})` : p.name)).join(', ')
      const failTail = failed ? `, ${failed} failed` : ''
      return {
        ok: created.length > 0,
        message: created.length
          ? `Created ${created.length} products for "${a.theme}"${failTail}: ${links}`
          : `No products were created for "${a.theme}".`,
        href: '/products',
      }
    },
  },
  {
    name: 'summarize',
    description: `Summarize the data in a section (counts + a sample). section defaults to the current view; one of: ${SECTIONS.join(', ')}.`,
    params: { type: 'object', properties: { section: { type: 'string', enum: SECTIONS } } },
    run: async (host, a) => {
      const section = a.section || host.currentSection()
      const summary = await host.summarize(section)
      return { ok: true, message: summary }
    },
  },
]

const BY_NAME = new Map(COMMANDS.map((c) => [c.name, c]))

/** Coerce the model's free-form `products[]` (JSON objects) into typed ProductSpecs.
 *  Skips anything without a usable name; tolerates string or number prices. */
export function readSpecs(raw: unknown): ProductSpec[] {
  if (!Array.isArray(raw)) return []
  const specs: ProductSpec[] = []
  for (const item of raw) {
    if (!item || typeof item !== 'object') continue
    const o = item as Record<string, unknown>
    const name = typeof o.name === 'string' ? o.name.trim() : ''
    if (!name) continue
    const priceRaw = o.priceUsd
    const priceUsd =
      typeof priceRaw === 'number'
        ? priceRaw
        : typeof priceRaw === 'string' && priceRaw.trim() !== ''
          ? Number(priceRaw)
          : undefined
    specs.push({
      name,
      description: typeof o.description === 'string' ? o.description : undefined,
      priceUsd: priceUsd != null && Number.isFinite(priceUsd) ? priceUsd : undefined,
      sku: typeof o.sku === 'string' ? o.sku : undefined,
      status: typeof o.status === 'string' ? o.status : undefined,
    })
  }
  return specs
}

// ── validation ───────────────────────────────────────────────────────────────
function validateArgs(
  schema: JsonSchema,
  raw: RawCommand,
): { ok: true; args: Record<string, string> } | { ok: false; error: string } {
  const args: Record<string, string> = {}
  for (const [key, spec] of Object.entries(schema.properties)) {
    const v = raw[key]
    if (v === undefined || v === null || v === '') continue
    const val = String(v)
    if (spec.enum && !spec.enum.includes(val)) {
      return { ok: false, error: `${key} must be one of ${spec.enum.join(', ')}` }
    }
    args[key] = val
  }
  for (const req of schema.required ?? []) {
    if (!(req in args)) return { ok: false, error: `missing ${req}` }
  }
  return { ok: true, args }
}

async function runOne(a: RawCommand, host: AppHost): Promise<CommandLogEntry> {
  const type = typeof a?.type === 'string' ? a.type.trim() : ''
  const cmd = BY_NAME.get(type)
  if (!cmd) return { ok: false, message: `Unknown command "${type || '?'}"` }
  const v = validateArgs(cmd.params, a)
  if (!v.ok) return { ok: false, message: `${cmd.name}: ${v.error}` }
  // `raw` is the FULL action so a command can read non-scalar params
  // (e.g. generate_catalog's `products[]`) the scalar validator skips.
  const ctx: RunCtx = { label: (k) => k, raw: a }
  try {
    return await cmd.run(host, v.args, ctx)
  } catch {
    return { ok: false, message: `Could not run ${cmd.name}` }
  }
}

// Per-call ceilings on model-emitted work: at most MAX_ACTIONS steps, minting at
// most MAX_CREATES records total (the generate_catalog cap can't be defeated by a
// long actions[] since every create charges against ONE shared budget here).
const MAX_ACTIONS = 8
const MAX_CREATES = 24

function clamp(n: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(hi, n))
}

/** Records this action will try to create — what it charges against the creates budget. */
function createCost(a: RawCommand): number {
  const type = typeof a?.type === 'string' ? a.type.trim() : ''
  if (type === 'generate_catalog') return clamp(Math.round(Number(a.count)) || 1, 1, 24)
  if (type.startsWith('create_')) return 1
  return 0
}

/** ONE dispatcher: gate → validate → execute → log, per action. Never throws. */
export async function dispatch(actions: RawCommand[], host: AppHost): Promise<CommandLogEntry[]> {
  if (!actions?.length) return []
  if (!host.isAuthed()) return [{ ok: false, message: 'Sign in to let the assistant act.' }]
  const log: CommandLogEntry[] = []
  let creates = 0
  for (const a of actions.slice(0, MAX_ACTIONS)) {
    const cost = createCost(a)
    if (cost > 0 && creates + cost > MAX_CREATES) {
      log.push({ ok: false, message: `Reached the ${MAX_CREATES}-record create limit; stopped before "${a.type ?? '?'}".` })
      break
    }
    creates += cost
    log.push(await runOne(a, host))
  }
  return log
}

// ── contract shipped to the model ────────────────────────────────────────────
export function commandManifest() {
  return COMMANDS.map((c) => ({ name: c.name, description: c.description, params: c.params }))
}

export function buildSystemPrompt(context: string): string {
  const cmds = commandManifest()
    .map((c) => `- ${c.name}(${Object.keys(c.params.properties).join(', ')}): ${c.description}`)
    .join('\n')
  return [
    'You are the AGENTIC assistant embedded in the Hanzo Commerce admin dashboard.',
    'You help the merchant run their store and you can ACT on it, not just describe it.',
    'Be concise and concrete.',
    '',
    'You can drive the UI with these commands (emit them only when the user actually asks you to act):',
    cmds,
    '',
    'Acting rules:',
    '- When the merchant asks to add/create/make/set up something (a product, collection,',
    '  store, or a whole catalog), OFFER to do it: emit the matching create command inline',
    '  rather than only navigating there. Invent sensible defaults for anything they omit',
    '  (e.g. a fitting SKU, a short description, a reasonable price) and say what you chose.',
    '- For "generate/seed a catalog" or "add N products", use generate_catalog and INVENT the',
    '  full product specs in `products` — do not ask the merchant to enumerate them.',
    '- Prefer one decisive action over a clarifying question when the intent is clear.',
    '- Otherwise (pure questions, or the user only wants to look), keep "actions" empty.',
    '',
    'Live context:',
    context,
    '',
    'Respond with ONE JSON object and nothing else:',
    '{"reply": string, "actions": Array<{"type": string, ...params}>}',
    '"reply" is your natural-language answer (mention the record you created). Use exact',
    'command names and parameter names above. Never wrap the JSON in markdown fences.',
  ].join('\n')
}

// ── tolerant parse of the model reply (studio copilot style) ─────────────────
export function parseAssistant(content: string): { reply: string; actions: RawCommand[] } {
  const text = (content ?? '').trim()
  const tryParse = (s: string): { reply: string; actions: RawCommand[] } | null => {
    try {
      const o = JSON.parse(s)
      if (o && typeof o === 'object' && ('reply' in o || 'actions' in o)) {
        const actions = Array.isArray(o.actions) ? (o.actions as RawCommand[]) : []
        const reply = typeof o.reply === 'string' ? o.reply : ''
        return { reply, actions }
      }
    } catch {
      /* fall through */
    }
    return null
  }
  // 1. whole content
  const whole = tryParse(text)
  if (whole) return whole
  // 2. fenced ```json ... ```
  const fence = text.match(/```(?:json)?\s*([\s\S]*?)```/i)
  if (fence) {
    const inner = tryParse(fence[1].trim())
    if (inner) return inner
  }
  // 3. first {...} block
  const brace = text.match(/\{[\s\S]*\}/)
  if (brace) {
    const inner = tryParse(brace[0])
    if (inner) return inner
  }
  // 4. plain prose
  return { reply: text, actions: [] }
}
