<p align="center">
  <a href="https://hanzo.ai">
    <img alt="Hanzo Commerce" src="https://hanzo.ai/logo.svg" width="200">
  </a>
</p>

<h1 align="center">
  Hanzo Commerce Storefront
</h1>

<p align="center">
A Next.js 15 storefront powered by the Hanzo Commerce API at api.commerce.hanzo.ai.</p>

### Prerequisites

To use the Hanzo Commerce Storefront, you need access to the Hanzo Commerce API.

The storefront connects to `https://api.commerce.hanzo.ai` by default, or you can set `HANZO_COMMERCE_API_URL` to point to a local or custom backend.

# Overview

The Hanzo Commerce Storefront is built with:

- [Next.js](https://nextjs.org/)
- [Tailwind CSS](https://tailwindcss.com/)
- [Typescript](https://www.typescriptlang.org/)
- [Hanzo Commerce](https://hanzo.ai/)

Features include:

- Full ecommerce support:
  - Product Detail Page
  - Product Overview Page
  - Product Collections
  - Cart
  - Checkout with Stripe
  - User Accounts
  - Order Details
- Full Next.js 15 support:
  - App Router
  - Next fetching/caching
  - Server Components
  - Server Actions
  - Streaming
  - Static Pre-Rendering

# Quickstart

### Setting up the environment variables

Navigate into your projects directory and get your environment variables ready:

```shell
mv .env.template .env.local
```

### Install dependencies

Use Yarn to install all dependencies.

```shell
yarn
```

### Start developing

You are now ready to start up your project.

```shell
yarn dev
```

### Open the code and start customizing

Your site is now running at http://localhost:8000!

# Payment integrations

By default this starter supports the following payment integrations

- [Stripe](https://stripe.com/)

To enable the integrations you need to add the following to your `.env.local` file:

```shell
NEXT_PUBLIC_STRIPE_KEY=<your-stripe-public-key>
```

# Resources

## Learn more about Hanzo Commerce

- [Website](https://hanzo.ai/)
- [GitHub](https://github.com/hanzoai)

## Learn more about Next.js

- [Website](https://nextjs.org/)
- [GitHub](https://github.com/vercel/next.js)
- [Documentation](https://nextjs.org/docs)
