"use client"

import clsx from "clsx"
import React, { useState } from "react"

type Tab = {
  title: string
  code: string
  lang: string
  description: string
  linkTitle: string
  linkHref: string
}

const HomepageCodeTabs = () => {
  const [selectedTabIndex, setSelectedTabIndex] = useState(0)

  const tabs: Tab[] = [
    {
      title: "Create API Route",
      description:
        "Expose custom features with REST API routes, then consume them from your client applications.",
      linkTitle: "API Routes",
      linkHref: "/learn/fundamentals/api-routes",
      lang: "ts",
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
      lang: "ts",
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
      lang: "ts",
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
      lang: "ts",
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
      lang: "ts",
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
      lang: "tsx",
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

  return (
    <div
      className={clsx(
        "w-full border-b border-gray-200 dark:border-gray-800",
        "flex gap-0 flex-col lg:flex-row"
      )}
    >
      <div className="w-full lg:w-1/2 flex flex-col gap-0 border-r border-gray-200 dark:border-gray-800">
        {tabs.map((tab, index) => (
          <React.Fragment key={index}>
            <button
              className={clsx(
                "px-8 py-6 appearance-none text-left",
                "flex flex-col gap-2 group",
                "border-b border-gray-100 dark:border-gray-800 last:border-b-0"
              )}
              onClick={() => setSelectedTabIndex(index)}
            >
              <div className="flex items-center gap-2">
                <span
                  className={clsx(
                    "text-xs font-mono group-hover:text-blue-600 dark:group-hover:text-blue-400",
                    index === selectedTabIndex && "text-blue-600 dark:text-blue-400",
                    index !== selectedTabIndex && "text-gray-400"
                  )}
                >
                  [ {index + 1} ]
                </span>
                <span className="text-sm font-medium text-gray-900 dark:text-gray-100">
                  {tab.title}
                </span>
              </div>
              {index === selectedTabIndex && (
                <p className="text-gray-500 text-sm">
                  {tab.description}
                </p>
              )}
            </button>
          </React.Fragment>
        ))}
      </div>
      <div
        className={clsx(
          "w-full lg:w-1/2 p-8 flex flex-col gap-4 justify-center bg-gray-50 dark:bg-gray-900 relative"
        )}
      >
        <pre className="overflow-auto text-sm font-mono bg-gray-900 dark:bg-gray-950 text-gray-100 p-6 rounded-lg">
          <code>{tabs[selectedTabIndex].code}</code>
        </pre>
        <div className="flex gap-2 items-center text-sm text-gray-500">
          <span className="px-2 py-0.5 rounded border border-gray-200 dark:border-gray-700">
            {tabs[selectedTabIndex].linkTitle}
          </span>
          <a
            href={tabs[selectedTabIndex].linkHref}
            className="text-blue-600 dark:text-blue-400 hover:underline"
          >
            Learn more
          </a>
        </div>
      </div>
    </div>
  )
}

export default HomepageCodeTabs
