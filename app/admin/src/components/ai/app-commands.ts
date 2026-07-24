// ─────────────────────────────────────────────────────────────────────────────
// ONE control surface for the AI dock (decomplected, after world.hanzo.ai's
// app-commands): a PORT the app implements, a DATA table of ops (single source of
// truth), and ONE validated dispatcher. The backend/MCP tool contract is DERIVED
// from `commandManifest()` — never duplicated.
//
// v1 ships only ops the FE can TRULY perform (no fake tools):
//   navigate · search · open · summarize
// BE/MCP commerce tools (create/edit product, refund order, …) are a sequenced
// follow-up: add an AppCommand entry here + implement it on AppHost. Nothing else
// changes — the prompt, manifest, validation and dispatch all read this table.
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

// PORT — the narrow capability surface the assistant may drive. The dock implements it.
export interface AppHost {
  isAuthed(): boolean
  currentSection(): string
  navigate(section: string): void
  /** Drive a list section's search (navigates there + filters by query). */
  filter(section: string, query: string): void
  /** Fetch real data for a section and return a short human summary. */
  summarize(section: string): Promise<string>
  createProduct(input: { name: string; sku: string; description?: string }): Promise<{ id?: string; name: string }>
}

export interface CommandResult { ok: boolean; message: string }
export type CommandLogEntry = CommandResult
export interface RunCtx { label: (k: string) => string }

interface JsonSchema {
  type: 'object'
  properties: Record<string, { type: string; enum?: readonly string[]; description?: string }>
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
    description: 'Create a draft product in the active store. Requires a name and SKU. Use only when the merchant explicitly asks to create it.',
    params: {
      type: 'object',
      properties: {
        name: { type: 'string', description: 'Product name' },
        sku: { type: 'string', description: 'Unique merchant SKU' },
        description: { type: 'string', description: 'Product description' },
      },
      required: ['name', 'sku'],
    },
    run: async (host, a) => {
      const product = await host.createProduct({ name: a.name, sku: a.sku, description: a.description })
      return { ok: true, message: `Created draft product "${product.name}"` }
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

async function runOne(a: RawCommand, host: AppHost, ctx: RunCtx): Promise<CommandLogEntry> {
  const type = typeof a?.type === 'string' ? a.type.trim() : ''
  const cmd = BY_NAME.get(type)
  if (!cmd) return { ok: false, message: `Unknown command "${type || '?'}"` }
  const v = validateArgs(cmd.params, a)
  if (!v.ok) return { ok: false, message: `${cmd.name}: ${v.error}` }
  try {
    return await cmd.run(host, v.args, ctx)
  } catch {
    return { ok: false, message: `Could not run ${cmd.name}` }
  }
}

/** ONE dispatcher: gate → validate → execute → log, per action. Never throws. */
export async function dispatch(actions: RawCommand[], host: AppHost): Promise<CommandLogEntry[]> {
  if (!actions?.length) return []
  if (!host.isAuthed()) return [{ ok: false, message: 'Sign in to let the assistant act.' }]
  const ctx: RunCtx = { label: (k) => k }
  const log: CommandLogEntry[] = []
  for (const a of actions) log.push(await runOne(a, host, ctx))
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
    'You are the assistant embedded in the Hanzo Commerce admin dashboard.',
    'Help the merchant understand and navigate their store. Be concise and concrete.',
    '',
    'You can drive the UI with these commands (only when the user actually asks you to act):',
    cmds,
    '',
    'Live context:',
    context,
    '',
    'Respond with ONE JSON object and nothing else:',
    '{"reply": string, "actions": Array<{"type": string, ...params}>}',
    '"reply" is your natural-language answer. Put commands in "actions" ONLY when the user',
    'asks you to navigate/search/open/summarize/create a product; otherwise "actions" must be []. Use exact',
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
