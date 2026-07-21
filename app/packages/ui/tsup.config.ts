import { defineConfig } from 'tsup'
import { readFile, writeFile, rm } from 'node:fs/promises'
import { resolve } from 'node:path'

// @hanzo/commerce-ui is the Medusa-lineage design system (Radix + cva + tailwind-merge).
// Its source uses `@/` path aliases (tsconfig `paths`) which cannot be consumed as source by
// a downstream bundler, so we PRE-BUILD to dist. tsup/esbuild resolves the `@/` aliases from
// tsconfig.json natively (no tsc-alias needed) and emits cjs + esm + one bundled .d.ts.
//
// RSC boundary — why we split per file and re-attach directives:
// The storefront (Next 15 App Router) imports this barrel from BOTH server and client
// components. Interactive components (Radix, hooks, react-dom/client) declare a module-level
// `"use client"`; server-safe primitives (Text, Heading, clx, …) do NOT — and server
// components legitimately CALL clx() during render, so it must stay a plain callable export,
// never a client reference. esbuild strips source directives when it bundles, and a single
// bundled barrel is all-or-nothing (marking it client would break clx on the server). So we
// emit ONE output file per source module (entry glob + `splitting`) — making `index.mjs` a
// thin re-export and isolating clx in its own directive-free chunk — then, in `onSuccess`,
// re-attach every source file's directive to the chunk it produced. (esbuild's own onEnd
// mutations don't survive tsup's write pipeline, so we tag the written files directly, keyed
// off the emitted metafile.) The public type surface is unchanged: dts is still bundled from
// the single `src/index.ts` barrel.

const rscDirective = /^\s*['"](use client|use server)['"]/

/** Read a source file's leading RSC directive, if any. Cached across the whole run. */
function makeDirectiveReader() {
  const cache = new Map<string, string | null>()
  return async (absPath: string): Promise<string | null> => {
    const hit = cache.get(absPath)
    if (hit !== undefined) return hit
    let directive: string | null = null
    try {
      const match = (await readFile(absPath, 'utf8')).slice(0, 300).match(rscDirective)
      if (match) directive = match[1]
    } catch {
      // Non-source input (e.g. a bundled dependency); no directive to carry.
    }
    cache.set(absPath, directive)
    return directive
  }
}

export default defineConfig({
  entry: [
    'src/**/*.{ts,tsx}',
    '!src/**/*.stories.{ts,tsx}',
    '!src/**/*.spec.{ts,tsx}',
    '!src/**/*.d.ts',
  ],
  format: ['cjs', 'esm'],
  dts: { entry: 'src/index.ts' },
  sourcemap: true,
  clean: true,
  treeshake: true,
  splitting: true,
  metafile: true,
  // react/react-dom are peers; all runtime deps (radix-ui, @tanstack, cva, sonner,
  // @hanzo/commerce-icons, …) are auto-externalized by tsup from package.json.
  external: ['react', 'react-dom'],
  esbuildOptions(options) {
    // Automatic JSX runtime works whether or not a file imports React (some don't).
    options.jsx = 'automatic'
  },
  async onSuccess() {
    const directiveOf = makeDirectiveReader()
    for (const format of ['esm', 'cjs'] as const) {
      const metaPath = resolve('dist', `metafile-${format}.json`)
      let meta: { outputs: Record<string, { inputs: Record<string, unknown> }> }
      try {
        meta = JSON.parse(await readFile(metaPath, 'utf8'))
      } catch {
        continue
      }
      for (const [out, { inputs }] of Object.entries(meta.outputs)) {
        if (!/\.(mjs|cjs|js)$/.test(out)) continue
        const directives = new Set<string>()
        for (const input of Object.keys(inputs)) {
          const directive = await directiveOf(resolve(input))
          if (directive) directives.add(directive)
        }
        if (directives.size === 0) continue
        const outPath = resolve(out)
        const code = await readFile(outPath, 'utf8')
        if (rscDirective.test(code)) continue
        const banner = [...directives].map((d) => `"${d}";`).join('\n') + '\n'
        await writeFile(outPath, banner + code)
      }
      // The metafile is a build artifact, not a shipped file.
      await rm(metaPath, { force: true })
    }
  },
})
