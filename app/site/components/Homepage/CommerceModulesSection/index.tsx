import clsx from "clsx"
import Link from "next/link"

type SectionProps = {
  title: string
  modules: {
    name: string
    description: string
    link: string
  }[]
}

const HomepageCommerceModulesSection = () => {
  const sections: SectionProps[] = [
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

  return (
    <div className="w-full border-b border-gray-200 dark:border-gray-800">
      <div className="p-8 flex gap-4 border-b border-gray-200 dark:border-gray-800">
        <h2 className="text-2xl font-bold text-gray-900 dark:text-gray-100 min-w-max">
          Commerce Modules
        </h2>
      </div>
      <div className="flex flex-wrap gap-0">
        {sections.map((section, index) => (
          <div
            key={index}
            className={clsx(
              "py-8 w-full sm:w-1/2 lg:w-1/3",
              "flex flex-col gap-4 items-start",
              "border-gray-200 dark:border-gray-800",
              "border-b",
              index === 3 && "lg:border-b-0",
              index > 3 && "sm:border-b-0",
              index % 3 !== 2 && "border-r",
              index === 2 && "border-r lg:border-r-0"
            )}
          >
            <span className="text-xs font-medium text-gray-500 uppercase tracking-wide px-8">
              {section.title}
            </span>
            {section.modules.map((module, modIndex) => (
              <div
                key={modIndex}
                className="flex flex-col gap-0 group relative px-8 w-full"
              >
                <span className="absolute top-0 left-0 lg:-left-px w-[2px] h-full bg-transparent group-hover:bg-blue-600" />
                <span className="text-sm font-medium text-gray-900 dark:text-gray-100">
                  {module.name}
                </span>
                <p className="text-xs text-gray-500">
                  {module.description}
                </p>
                <Link
                  href={module.link}
                  className="absolute left-0 top-0 w-full h-full opacity-0"
                >
                  Learn more
                </Link>
              </div>
            ))}
          </div>
        ))}
        <div
          className={clsx(
            "p-8 w-full sm:w-1/2 lg:w-1/3",
            "flex flex-col gap-4",
            "bg-gray-50 dark:bg-gray-900"
          )}
        >
          <div className="flex flex-col">
            <span className="text-sm font-semibold text-gray-900 dark:text-gray-100">
              Updates delivered monthly
            </span>
            <span className="text-xs text-gray-500">
              Get the latest product news and behind the scenes updates.
            </span>
          </div>
        </div>
      </div>
    </div>
  )
}

export default HomepageCommerceModulesSection
