const HomepageFooter = () => {
  return (
    <div className="p-8 w-full">
      <div className="flex flex-col gap-4 items-center text-center">
        <p className="text-sm text-gray-500">
          Was this page helpful?
        </p>
        <div className="flex gap-4 text-sm">
          <a
            href="https://github.com/hanzoai/commerce"
            className="text-gray-500 hover:text-gray-900 dark:hover:text-gray-100"
          >
            GitHub
          </a>
          <a
            href="https://discord.gg/hanzoai"
            className="text-gray-500 hover:text-gray-900 dark:hover:text-gray-100"
          >
            Discord
          </a>
          <a
            href="https://hanzo.ai"
            className="text-gray-500 hover:text-gray-900 dark:hover:text-gray-100"
          >
            hanzo.ai
          </a>
        </div>
        <p className="text-xs text-gray-400">
          &copy; {new Date().getFullYear()} Hanzo Industries. All rights reserved.
        </p>
      </div>
    </div>
  )
}

export default HomepageFooter
