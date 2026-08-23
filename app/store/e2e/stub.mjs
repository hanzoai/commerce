#!/usr/bin/env node
/**
 * The JSON stub the storefront is driven against when the check needs a
 * browser and not a backend: `node e2e/stub.mjs` then `next start` with
 * HANZO_COMMERCE_API_URL=http://127.0.0.1:9800. Enough /store/* surface for
 * the home, shop-all, product and cart pages to render fully styled — which
 * is exactly what `gui-css-check --render` measures (Gui drops unknown props
 * silently, so only a rendered page proves anything).
 */
import { createServer } from "node:http"

const region = {
  id: "reg_us",
  name: "United States",
  currency_code: "usd",
  countries: [{ id: "us", iso_2: "us", iso_3: "usa", display_name: "United States", region_id: "reg_us" }],
}

const price = (amount) => ({
  calculated_amount: amount,
  original_amount: amount,
  currency_code: "usd",
  calculated_price: { price_list_type: "sale" },
})

const product = (n) => ({
  id: `prod_${n}`,
  title: `Sample Product ${n}`,
  subtitle: null,
  handle: `sample-${n}`,
  description: "A product the stub serves so the storefront draws.",
  thumbnail: null,
  images: [],
  collection_id: "col_1",
  options: [{ id: `opt_${n}`, title: "Size", values: [{ id: `optv_${n}`, value: "One Size" }] }],
  tags: [],
  variants: [
    {
      id: `var_${n}`,
      title: "One Size",
      manage_inventory: false,
      allow_backorder: true,
      options: [{ option_id: `opt_${n}`, value: "One Size" }],
      calculated_price: price(1900 + n * 100),
    },
  ],
})

const products = [1, 2, 3, 4].map(product)
const collection = { id: "col_1", handle: "featured", title: "Featured", products }

const routes = {
  "/store/regions": { regions: [region] },
  "/store/regions/reg_us": { region },
  "/store/collections": { collections: [collection], count: 1 },
  "/store/collections/col_1": { collection },
  "/store/product-categories": { product_categories: [] },
  "/store/products": { products, count: products.length },
  "/store/customers/me": null, // 401 below
}

createServer((req, res) => {
  const path = new URL(req.url, "http://x").pathname
  const body =
    routes[path] ??
    (path.startsWith("/store/products/") ? { product: products[0] } : undefined)
  if (path === "/store/customers/me") {
    res.writeHead(401, { "content-type": "application/json" })
    return res.end(JSON.stringify({ message: "unauthorized" }))
  }
  if (body === undefined) {
    res.writeHead(404, { "content-type": "application/json" })
    return res.end(JSON.stringify({ message: `stub: no route ${path}` }))
  }
  res.writeHead(200, { "content-type": "application/json" })
  res.end(JSON.stringify(body))
}).listen(9800, () => console.log("commerce stub on :9800"))
