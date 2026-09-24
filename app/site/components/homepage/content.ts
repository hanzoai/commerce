/**
 * What the homepage says. Content only — no markup, no styling.
 *
 * Splitting it out is what let the page itself collapse from eleven forked
 * marketing components (each with its own Tailwind layout and its own copy of
 * a section header) to one composition of shared shapes.
 */

export type LinkItem = { text: string; link: string }

export const LINK_GROUPS: { tag: string; links: LinkItem[] }[] = [
  {
    tag: "Customize Hanzo Commerce",
    links: [
      { link: "/learn/installation", text: "Install Hanzo Commerce" },
      {
        link: "https://commerce.hanzo.ai/cloud/sign-up",
        text: "Deploy to Cloud",
      },
      {
        link: "https://commerce.hanzo.ai/resources/integrations",
        text: "Browse integrations",
      },
    ],
  },
  {
    tag: "Admin Development",
    links: [
      { link: "/learn/fundamentals/admin/widgets", text: "Build a UI widget" },
      { link: "/learn/fundamentals/admin/ui-routes", text: "Add a UI route" },
      { link: "https://commerce.hanzo.ai/ui", text: "Browse the UI library" },
    ],
  },
  {
    tag: "Storefront Development",
    links: [
      {
        link: "https://commerce.hanzo.ai/resources/nextjs-starter",
        text: "Explore storefront starter",
      },
      {
        link: "https://commerce.hanzo.ai/resources/storefront-development",
        text: "Build custom storefront",
      },
      {
        link: "https://commerce.hanzo.ai/learn/introduction/build-with-llms-ai",
        text: "Use agent skills",
      },
    ],
  },
  {
    tag: "Hanzo Cloud",
    links: [
      {
        link: "https://commerce.hanzo.ai/cloud/projects",
        text: "Deploy from GitHub",
      },
      {
        link: "https://commerce.hanzo.ai/cloud/environments/preview",
        text: "Preview environments",
      },
      {
        link: "https://commerce.hanzo.ai/cloud/emails",
        text: "Hanzo Commerce Emails",
      },
    ],
  },
  {
    tag: "Agentic Development",
    links: [
      { link: "https://hanzo.ai", text: "Build with Hanzo AI" },
      {
        link: "https://commerce.hanzo.ai/learn/introduction/build-with-llms-ai",
        text: "Agent Skills",
      },
      {
        link: "https://commerce.hanzo.ai/learn/introduction/build-with-llms-ai#mcp-remote-server",
        text: "Hanzo Commerce Docs MCP",
      },
    ],
  },
]

export type CodeSample = {
  title: string
  description: string
  linkTitle: string
  linkHref: string
  code: string
}

export const CODE_SAMPLES: CodeSample[] = [
  {
    title: "Create API Route",
    description:
      "Expose custom features with REST API routes, then consume them from your client applications.",
    linkTitle: "API Routes",
    linkHref: "/learn/fundamentals/api-routes",
    code: `export async function GET(
  req: HanzoRequest,
  res: HanzoResponse
) {
  const query = req.scope.resolve("query")

  const { data } = await query.graph({
    entity: "company",
    fields: ["id", "name"],
    filters: { name: "ACME" },
  })

  res.json({
    companies: data
  })
}`,
  },
  {
    title: "Build Workflows",
    description:
      "Build flows as a series of steps, with retry mechanisms and tracking of each steps' status.",
    linkTitle: "Workflows",
    linkHref: "/learn/fundamentals/workflows",
    code: `const handleDeliveryWorkflow = createWorkflow(
  "handle-delivery",
  function (input: WorkflowInput) {
    notifyRestaurantStep(input.delivery_id)

    const order = createOrderStep(input.delivery_id)

    createFulfillmentStep(order)

    awaitDeliveryStep()

    return new WorkflowResponse("Delivery completed")
  }
)`,
  },
  {
    title: "Add a Data Model",
    description:
      "Create data models that represent tables in the database using the Data Model Language.",
    linkTitle: "DML",
    linkHref: "/learn/fundamentals/modules#1-create-data-model",
    code: `const DigitalProduct = model.define("digital_product",
{
  id: model.id().primaryKey(),
  name: model.text(),
  medias: model.hasMany(() => DigitalProductMedia, {
    mappedBy: "digitalProduct"
  })
})
.cascades({
  delete: ["medias"]
})`,
  },
  {
    title: "Build a Custom Module",
    description:
      "Build custom modules with commerce or architectural features and use them in API routes or workflows.",
    linkTitle: "Modules",
    linkHref: "/learn/fundamentals/modules",
    code: `class DigitalProductService extends HanzoService({
  DigitalProduct,
}) {
  async authorizeLicense() {
    console.log("License authorized!")
  }
}

export async function POST(
  req: HanzoRequest,
  res: HanzoResponse
) {
  const moduleService = req.scope.resolve(
    "digitalProduct"
  )

  await moduleService.authorizeLicense()

  res.json({ success: true })
}`,
  },
  {
    title: "Subscribe to Events",
    description:
      "Handle events emitted by the application to perform custom actions.",
    linkTitle: "Subscribers",
    linkHref: "/learn/fundamentals/events-and-subscribers",
    code: `async function orderPlaced({
  container,
}: SubscriberArgs) {
  const moduleService = container.resolve(
    Modules.NOTIFICATION
  )

  await moduleService.createNotifications({
    to: "customer@gmail.com",
    channel: "email",
    template: "order-placed"
  })
}

export const config: SubscriberConfig = {
  event: "order.placed",
}`,
  },
  {
    title: "Customize Admin",
    description:
      "Inject widgets into predefined zones in the Admin, or add new pages.",
    linkTitle: "Admin Widgets",
    linkHref: "/learn/fundamentals/admin/widgets",
    code: `const ProductBrandWidget = () => {
  const [brand, setBrand] = useState({
    name: "Acme"
  })

  return (
    <Container>
      <Heading level="h2">Brand</Heading>
      {brand && <span>Name: {brand.name}</span>}
    </Container>
  )
}

export const config = defineWidgetConfig({
  zone: "product.details.before",
})`,
  },
]

export type Recipe = { title: string; description: string; link: string }

export const RECIPES: Recipe[] = [
  {
    title: "Marketplace",
    description: "Build a marketplace with multiple vendors.",
    link: "https://commerce.hanzo.ai/resources/recipes/marketplace/examples/vendors",
  },
  {
    title: "ERP",
    description:
      "Integrate an ERP system to manage custom product prices, purchase rules, syncing orders, and more.",
    link: "https://commerce.hanzo.ai/resources/recipes/erp",
  },
  {
    title: "Bundled Products",
    description:
      "Sell products as bundles with Admin and storefront customizations.",
    link: "https://commerce.hanzo.ai/resources/recipes/bundled-products/examples/standard",
  },
  {
    title: "Subscriptions",
    description: "Implement a subscription-based commerce store.",
    link: "https://commerce.hanzo.ai/resources/recipes/subscriptions/examples/standard",
  },
  {
    title: "Restaurant-Delivery",
    description:
      "Build a restaurant marketplace inspired by UberEats, with real-time delivery handling.",
    link: "https://commerce.hanzo.ai/resources/recipes/marketplace/examples/restaurant-delivery",
  },
  {
    title: "Digital Products",
    description: "Sell digital products with custom fulfillment.",
    link: "https://commerce.hanzo.ai/resources/recipes/digital-products/examples/standard",
  },
]

export type CommerceModule = { name: string; description: string; link: string }

export const MODULE_GROUPS: { title: string; modules: CommerceModule[] }[] = [
  {
    title: "Cart & Purchase",
    modules: [
      {
        name: "Cart",
        description: "Add to cart, checkout, and totals",
        link: "/resources/commerce-modules/cart",
      },
      {
        name: "Payment",
        description: "Process any payment type",
        link: "/resources/commerce-modules/payment",
      },
      {
        name: "Customer",
        description: "Customer and group management",
        link: "/resources/commerce-modules/customer",
      },
    ],
  },
  {
    title: "Merchandising",
    modules: [
      {
        name: "Pricing",
        description: "Configurable pricing engine",
        link: "/resources/commerce-modules/pricing",
      },
      {
        name: "Promotion",
        description: "Discounts and promotions",
        link: "/resources/commerce-modules/promotion",
      },
      {
        name: "Product",
        description: "Variants, categories, and bulk edits",
        link: "/resources/commerce-modules/product",
      },
    ],
  },
  {
    title: "Fulfillment",
    modules: [
      {
        name: "Order",
        description: "Omnichannel order management",
        link: "/resources/commerce-modules/order",
      },
      {
        name: "Inventory",
        description: "Multi-warehouse and reservations",
        link: "/resources/commerce-modules/inventory",
      },
      {
        name: "Fulfillment",
        description: "Order fulfillment and shipping",
        link: "/resources/commerce-modules/fulfillment",
      },
      {
        name: "Stock Location",
        description: "Locations of stock-kept items",
        link: "/resources/commerce-modules/stock-location",
      },
    ],
  },
  {
    title: "Regions & Channels",
    modules: [
      {
        name: "Region",
        description: "Cross-border commerce",
        link: "/resources/commerce-modules/region",
      },
      {
        name: "Sales Channel",
        description: "Omnichannel sales",
        link: "/resources/commerce-modules/sales-channel",
      },
      {
        name: "Tax",
        description: "Granular tax control",
        link: "/resources/commerce-modules/tax",
      },
      {
        name: "Currency",
        description: "Multi-currency support",
        link: "/resources/commerce-modules/currency",
      },
    ],
  },
  {
    title: "User Access",
    modules: [
      {
        name: "API Keys",
        description: "Store and admin access",
        link: "/resources/commerce-modules/api-key",
      },
      {
        name: "User Module",
        description: "Admin user management",
        link: "/resources/commerce-modules/user",
      },
      {
        name: "Auth",
        description: "Integrate authentication methods",
        link: "/resources/commerce-modules/auth",
      },
    ],
  },
]
