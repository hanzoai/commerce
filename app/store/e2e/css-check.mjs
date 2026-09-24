#!/usr/bin/env node
/**
 * The storefront's class-coverage gate, end to end: stub backend up, store
 * BUILT against it (generateStaticParams needs a live /store/*, and a page
 * prerendered without one cannot SSR later — DYNAMIC_SERVER_USAGE), store
 * served, `gui-css-check --render` over the pages a shopper actually sees.
 * A green `next build` cannot see a class with no rule — Gui drops unknown
 * style props silently and a browser renders unknown classes as nothing — so
 * the only honest gate is a rendered page. Needs Playwright's chromium
 * (`npx playwright install chromium`). One entry, owns its whole world.
 */
import { spawn } from "node:child_process"

const STUB = 9800
const PORT = 8000
const env = {
  ...process.env,
  HANZO_COMMERCE_API_URL: `http://127.0.0.1:${STUB}`,
  NEXT_PUBLIC_HANZO_COMMERCE_KEY: process.env.NEXT_PUBLIC_HANZO_COMMERCE_KEY || "pk_stub",
  NEXT_PUBLIC_DEFAULT_REGION: "us",
}

const ROUTES = ["/us", "/us/store", "/us/products/sample-1", "/us/cart"]

// Each child leads its own process group so teardown can kill the whole
// tree — `pnpm exec next start` re-spawns, and killing only pnpm leaves a
// grandchild holding the port and this script's event loop open forever.
const children = []
const run = (cmd, args) => {
  const c = spawn(cmd, args, { env, stdio: ["ignore", "inherit", "inherit"], detached: true })
  children.push(c)
  return c
}
const stop = () => {
  for (const c of children) {
    try {
      process.kill(-c.pid, "SIGTERM")
    } catch {}
  }
}

const up = async (url, tries = 60) => {
  for (let i = 0; i < tries; i++) {
    try {
      await fetch(url, { redirect: "manual" })
      return
    } catch {
      await new Promise((r) => setTimeout(r, 500))
    }
  }
  throw new Error(`${url} never came up`)
}

try {
  run("node", ["e2e/stub.mjs"])
  await up(`http://127.0.0.1:${STUB}/store/regions`)

  const build = run("pnpm", ["run", "build"])
  const built = await new Promise((res) => build.on("exit", res))
  if (built !== 0) {
    stop()
    process.exit(built ?? 1)
  }

  run("pnpm", ["exec", "next", "start", "-p", String(PORT)])
  await up(`http://127.0.0.1:${PORT}/us`)

  const check = run("pnpm", [
    "exec",
    "gui-css-check",
    ...ROUTES.flatMap((r) => ["--render", `http://127.0.0.1:${PORT}${r}`]),
  ])
  const code = await new Promise((res) => check.on("exit", res))
  stop()
  process.exit(code ?? 1)
} finally {
  stop()
}
