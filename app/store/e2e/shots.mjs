#!/usr/bin/env node
/**
 * Rendered screenshots of the storefront, driven against the same JSON stub the
 * class-coverage check uses. `node e2e/shots.mjs <outDir>` builds, serves, and
 * captures each route at a phone and a desktop width — the two the card grids
 * step between, which is the thing a class-coverage pass cannot see.
 */
import { spawn } from "node:child_process"
import { mkdirSync } from "node:fs"
import { chromium } from "@playwright/test"

const OUT = process.argv[2] || "shots"
const STUB = 9800
const PORT = 8000
const env = {
  ...process.env,
  HANZO_COMMERCE_API_URL: `http://127.0.0.1:${STUB}`,
  NEXT_PUBLIC_HANZO_COMMERCE_KEY:
    process.env.NEXT_PUBLIC_HANZO_COMMERCE_KEY || "pk_stub",
  NEXT_PUBLIC_DEFAULT_REGION: "us",
}

const ROUTES = ["/us", "/us/store", "/us/products/sample-1", "/us/cart"]
const WIDTHS = [390, 1280]

const children = []
const run = (cmd, args) => {
  const c = spawn(cmd, args, {
    env,
    stdio: ["ignore", "ignore", "inherit"],
    detached: true,
  })
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
const up = async (url, tries = 120) => {
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
  mkdirSync(OUT, { recursive: true })
  run("node", ["e2e/stub.mjs"])
  await up(`http://127.0.0.1:${STUB}/store/regions`)

  const build = run("pnpm", ["run", "build"])
  if ((await new Promise((r) => build.on("exit", r))) !== 0) {
    stop()
    process.exit(1)
  }

  run("pnpm", ["exec", "next", "start", "-p", String(PORT)])
  await up(`http://127.0.0.1:${PORT}/us`)

  const browser = await chromium.launch()
  for (const width of WIDTHS) {
    const page = await browser.newPage({ viewport: { width, height: 900 } })
    for (const route of ROUTES) {
      await page.goto(`http://127.0.0.1:${PORT}${route}`, {
        waitUntil: "networkidle",
      })
      const name = route.replace(/\W+/g, "_").replace(/^_/, "")
      await page.screenshot({
        path: `${OUT}/${width}-${name}.png`,
        fullPage: true,
      })
      console.log(`${width}px ${route}`)
    }
    await page.close()
  }
  await browser.close()
  stop()
  process.exit(0)
} finally {
  stop()
}
