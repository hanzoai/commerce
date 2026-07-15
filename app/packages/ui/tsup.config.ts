import { defineConfig } from 'tsup'

// @hanzo/commerce-ui is the Medusa-lineage design system (Radix + cva + tailwind-merge).
// Its source uses `@/` path aliases (tsconfig `paths`) which cannot be consumed as source
// by a downstream bundler, so we PRE-BUILD to dist. tsup/esbuild resolves the `@/` aliases
// from tsconfig.json natively (no tsc-alias needed) and emits cjs + esm + one bundled .d.ts.
//
// No `"use client"` banner is needed: in the admin (Next 15 `output: 'export'` SPA)
// every importer of this barrel is itself a client component, so the components land
// in the client graph transitively. (esbuild also strips a bundled module-level
// directive, so a banner would be a no-op warning anyway.)
export default defineConfig({
  entry: ['src/index.ts'],
  format: ['cjs', 'esm'],
  dts: true,
  sourcemap: true,
  clean: true,
  treeshake: true,
  splitting: false,
  // react/react-dom are peers; all runtime deps (radix-ui, @tanstack, cva, sonner,
  // @hanzo/commerce-icons, …) are auto-externalized by tsup from package.json.
  external: ['react', 'react-dom'],
  esbuildOptions(options) {
    // Automatic JSX runtime works whether or not a file imports React (some don't).
    options.jsx = 'automatic'
  },
})
