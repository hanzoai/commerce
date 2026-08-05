import clsx from "clsx"
import Link from "next/link"

type Card = {
  title: string
  description: string
  link: string
}

const HomepageRecipesSection = () => {
  const cards: Card[] = [
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

  return (
    <div className="w-full border-b border-gray-200 dark:border-gray-800">
      <div className="flex flex-col md:flex-row gap-0 justify-center border-b border-gray-200 dark:border-gray-800">
        <div
          className={clsx(
            "w-full md:w-1/2 lg:w-1/3 bg-gray-50 dark:bg-gray-900 p-8",
            "flex justify-center items-center",
            "md:border-r border-gray-200 dark:border-gray-800",
            "border-b md:border-b-0"
          )}
        >
          <div className="text-6xl font-bold text-gray-300 dark:text-gray-700">
            Recipes
          </div>
        </div>
        <div
          className={clsx(
            "w-full md:w-1/2 lg:w-2/3 py-16 px-8",
            "flex flex-col gap-3 justify-center"
          )}
        >
          <div className="flex gap-2 items-center text-sm text-gray-500">
            <span className="px-2 py-0.5 rounded border border-gray-200 dark:border-gray-700">
              Recipes
            </span>
            <a
              href="https://commerce.hanzo.ai/resources/recipes"
              className="text-blue-600 dark:text-blue-400 hover:underline"
            >
              View all
            </a>
          </div>
          <h2 className="text-2xl font-bold text-gray-900 dark:text-gray-100 lg:max-w-[450px]">
            Hanzo Commerce supports any business use case.
          </h2>
          <p className="text-base text-gray-600 dark:text-gray-400">
            These recipes show you how to build a use case by customizing and
            extending existing data models and features, or creating new ones.
          </p>
        </div>
      </div>
      <div className="flex flex-wrap gap-0 flex-col sm:flex-row">
        {cards.map((card, index) => (
          <div
            key={index}
            className={clsx(
              "w-full sm:w-1/2 md:w-1/3 p-8 flex gap-4 flex-col",
              "border-b last:!border-b-0",
              index >= 3 && "md:border-b-0",
              index >= 4 && "sm:border-b-0",
              index % 3 !== 2 && "border-r",
              index === 2 && "border-r md:border-r-0",
              "border-gray-200 dark:border-gray-800",
              "group relative hover:bg-gray-50 dark:hover:bg-gray-900 transition"
            )}
          >
            <div className="flex flex-col gap-1">
              <h3 className="text-sm font-semibold text-gray-900 dark:text-gray-100">
                {card.title}
              </h3>
              <p className="text-sm text-gray-500">
                {card.description}
              </p>
            </div>
            <Link
              href={card.link}
              className="absolute top-0 left-0 w-full h-full opacity-0"
            >
              Learn more
            </Link>
          </div>
        ))}
      </div>
    </div>
  )
}

export default HomepageRecipesSection
