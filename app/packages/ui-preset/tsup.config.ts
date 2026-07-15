import { defineConfig } from "tsup"
import path from "path"

export default defineConfig({
  entry: ["src/index.ts"],
  format: ["cjs", "esm"],
  tsconfig: path.resolve(__dirname, "tsconfig.json"),
  // A Tailwind preset is a plain JS config object consumed via `require()` in
  // tailwind.config.js — it has no TS consumers, so no .d.ts is needed. Emitting
  // one only breaks the build (the CJS `module.exports = preset` source needs
  // @types/node in the dts type-check). Skip it.
  dts: false,
  clean: true,
})
