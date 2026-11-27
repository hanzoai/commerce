// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package shopify

import "strings"

const appIndexHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta name="referrer" content="no-referrer">
    <meta name="shopify-api-key" content="%SHOPIFY_API_KEY%">
    <!-- Shopify requires App Bridge from its CDN, unbundled, first. -->
    <script src="https://cdn.shopify.com/shopifycloud/app-bridge.js"></script>
    <script src="https://cdn.shopify.com/shopifycloud/polaris-2.js"></script>
    <title>Hanzo</title>
  </head>
  <body>
    <s-page heading="Hanzo">
      <s-banner id="problem" tone="critical" hidden></s-banner>

      <s-section heading="Hanzo account">
        <s-paragraph id="account">Checking your store...</s-paragraph>
        <s-paragraph id="binding" hidden></s-paragraph>
        <s-button id="connect" variant="primary" hidden>Connect Hanzo account</s-button>
      </s-section>

      <s-section heading="Sync">
        <s-paragraph id="sync">Waiting for the first sync.</s-paragraph>
        <s-unordered-list id="kinds"></s-unordered-list>
        <s-unordered-list id="counters"></s-unordered-list>
      </s-section>

      <s-section heading="Storefront">
        <s-paragraph id="pixel">Pixel: off.</s-paragraph>
        <s-text-field id="key" label="Hanzo ingest key" details="The publishable pk- key from your Hanzo console. It can write events and read nothing."></s-text-field>
        <s-button id="save">Turn on pixel</s-button>
        <s-button id="embed" hidden>Turn on structured data in your theme</s-button>
      </s-section>

      <s-section heading="Consent">
        <s-paragraph>
          Hanzo sends marketing only to customers whose Shopify email consent
          is subscribed. Consent syncs from Shopify, and an unsubscribe in a
          Hanzo email is written back to the customer in Shopify. The pixel
          follows your store's cookie banner: no analytics consent, no events.
        </s-paragraph>
      </s-section>

      <s-section heading="Plan">
        <s-paragraph id="plan">No plan yet.</s-paragraph>
        <s-button id="plans" hidden>Choose a plan</s-button>
      </s-section>
    </s-page>
    <script type="module" src="./app/app.js"></script>
  </body>
</html>
`

const appJS = `import {api, view, key} from './view.js';

const $ = (id) => document.getElementById(id);

async function call(path, body) {
  const token = await shopify.idToken();
  const r = await fetch(api + path, {
    method: 'POST',
    headers: {authorization: 'Bearer ' + token, 'content-type': 'application/json'},
    body: JSON.stringify(body || {}),
  });
  const data = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(data.message || 'Hanzo is not reachable right now.');
  return data;
}

function renderList(el, rows) {
  if (!el) return;
  el.replaceChildren(...rows.map((t) => {
    const item = document.createElement('s-list-item');
    item.textContent = t;
    return item;
  }));
}

function show(status) {
  const v = view(status);
  $('account').textContent = v.account;
  if ($('binding')) {
    $('binding').textContent = v.binding;
    $('binding').hidden = !v.binding;
  }
  $('connect').hidden = !v.connect;
  $('sync').textContent = v.sync;
  renderList($('kinds'), v.kinds);
  renderList($('counters'), v.counters);
  $('pixel').textContent = v.pixel;
  $('embed').hidden = !v.embed;
  $('embed').onclick = () => open(v.embed, '_top');
  $('plan').textContent = v.plan;
  $('plans').hidden = !v.plans;
  $('plans').onclick = () => open(v.plans, '_top');
}

function fail(err) {
  const b = $('problem');
  b.textContent = err.message;
  b.hidden = false;
}

$('connect').onclick = async () => {
  try {
    const {url} = await call('/link');
    open(url, '_top');
  } catch (err) {
    fail(err);
  }
};

$('save').onclick = async () => {
  const v = $('key').value || '';
  if (!key(v)) return fail(new Error('That is not a publishable pk- key.'));
  try {
    show(await call('/pixel', {key: v.trim()}));
  } catch (err) {
    fail(err);
  }
};

call('/session').then(show, fail);
`

const viewJS = `export const api = '/v1/source/shopify';

const labels = {product: 'Products', customer: 'Customers', order: 'Orders'};

export function view(s) {
  const st = s || {};
  const linked = typeof st.org === 'string' && st.org !== '';
  const kinds = (Array.isArray(st.sync) ? st.sync : []).map((k) => {
    const name = labels[k.kind] || k.kind;
    const count = Number.isFinite(k.count) ? k.count : 0;
    const written = Number.isFinite(k.written) && k.written > 0 ? ', ' + k.written + ' pushed' : '';
    return name + ': ' + String(k.status || 'unknown').toLowerCase() + ', ' + count + ' read' + written;
  });
  const c = st.counters && typeof st.counters === 'object' ? st.counters : null;
  const counters = c ? [
    'Catalog: ' + (c.products_read || 0) + ' ingested, ' + (c.products_written || 0) + ' synced out',
    'Inventory: ' + (c.inventory_written || 0) + ' SKU stock updates pushed',
    'Orders: ' + (c.orders_read || 0) + ' ingested, ' + (c.orders_written || 0) + ' fulfillments/refunds pushed',
    'Customers & promos: ' + (c.customers_read || 0) + ' customers, ' + (c.coupons_written || 0) + ' coupons pushed',
  ] : [];
  const binding = linked && st.source_id
    ? 'Source: ' + st.source_id + ' · Mode: ' + (st.sync_mode || 'bidirectional') + ' · Location: ' + (st.stock_location_id || 'default')
    : '';
  return {
    account: linked
      ? st.shop + ' is connected to the Hanzo org ' + st.org + (st.source_id ? ' (' + st.source_id + ').' : '.')
      : (st.shop || 'This store') + ' is not connected to a Hanzo account yet.',
    binding,
    connect: !linked,
    sync: kinds.length ? 'Latest backfill from Shopify:' : 'No backfill has run yet.',
    kinds,
    counters,
    pixel: st.pixel ? 'Pixel: on.' : 'Pixel: off.',
    embed: typeof st.embed === 'string' && st.embed.startsWith('https://') ? st.embed : '',
    plan: st.plan && st.plan.active
      ? 'Plan: ' + st.plan.handle + '.'
      : 'No Shopify plan is active.',
    plans: typeof st.plans === 'string' && st.plans.startsWith('https://admin.shopify.com/') ? st.plans : '',
  };
}

export function key(v) {
  return typeof v === 'string' && /^pk-[A-Za-z0-9_-]+$/.test(v.trim());
}
`

func renderAppIndex(clientID string) string {
	return strings.ReplaceAll(appIndexHTML, "%SHOPIFY_API_KEY%", clientID)
}
