import clsx from "clsx"
import Link from "next/link"

const HomepageLinksSection = () => {
  const links: {
    tag: string
    links: {
      text: string
      link: string
    }[]
  }[] = [
    {
      tag: "Customize Hanzo Commerce",
      links: [
        {
          link: "/learn/installation",
          text: "Install Hanzo Commerce",
        },
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
        {
          link: "/learn/fundamentals/admin/widgets",
          text: "Build a UI widget",
        },
        {
          link: "/learn/fundamentals/admin/ui-routes",
          text: "Add a UI route",
        },
        {
          link: "https://commerce.hanzo.ai/ui",
          text: "Browse the UI library",
        },
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
        {
          link: "https://hanzo.ai",
          text: "Build with Hanzo AI",
        },
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

  return (
    <div className="w-full flex gap-0 flex-col md:flex-row flex-wrap border-b border-gray-200 dark:border-gray-800">
      {links.map((section, index) => (
        <div
          key={index}
          className={clsx(
            "p-8 flex justify-between flex-col w-full md:w-1/3 gap-6 md:min-h-[320px]",
            "border-b border-gray-200 dark:border-gray-800 md:border-b-0",
            index !== 2 && "md:border-r",
            index > 2 && "md:border-t"
          )}
        >
          <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">
            {section.tag}
          </span>
          <div className="flex flex-col gap-3">
            {section.links.map((link, linkIndex) => (
              <Link
                key={linkIndex}
                href={link.link}
                className="text-lg font-semibold text-gray-900 dark:text-gray-100 hover:underline hover:text-blue-600 dark:hover:text-blue-400"
              >
                {link.text}
              </Link>
            ))}
          </div>
        </div>
      ))}
      <div
        className={clsx(
          "p-8 flex justify-center items-center w-full md:w-1/3 gap-6 md:min-h-[320px]",
          "border-gray-200 dark:border-gray-800 md:border-t bg-gray-50 dark:bg-gray-900"
        )}
      >
        <div className="text-center">
          <span className="text-4xl font-bold">H</span>
          <p className="text-sm text-gray-500 mt-2">Hanzo Commerce</p>
        </div>
      </div>
    </div>
  )
}

export default HomepageLinksSection
