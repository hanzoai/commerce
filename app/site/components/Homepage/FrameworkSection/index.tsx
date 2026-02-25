import clsx from "clsx"
import FrameworkSectionIcon from "./Icon"

const HomepageFrameworkSection = () => {
  return (
    <div className="w-full flex flex-col md:flex-row gap-0 justify-center border-b border-gray-200 dark:border-gray-800">
      <div
        className={clsx(
          "w-full md:w-1/2 lg:w-1/3 bg-gray-50 dark:bg-gray-900 p-8 flex justify-center items-center",
          "md:border-r border-gray-200 dark:border-gray-800",
          "border-b md:border-b-0"
        )}
      >
        <FrameworkSectionIcon />
      </div>
      <div
        className={clsx(
          "w-full md:w-1/2 lg:w-2/3 py-16 px-8",
          "flex flex-col gap-3 justify-center"
        )}
      >
        <div className="flex gap-2 items-center text-sm text-gray-500">
          <span className="px-2 py-0.5 rounded border border-gray-200 dark:border-gray-700">
            Framework
          </span>
          <a
            href="/learn/fundamentals/framework"
            className="text-blue-600 dark:text-blue-400 hover:underline"
          >
            Learn more
          </a>
        </div>
        <h2 className="text-2xl font-bold text-gray-900 dark:text-gray-100 lg:max-w-[450px]">
          AI-powered commerce platform with a built-in framework for
          customizations.
        </h2>
        <p className="text-base text-gray-600 dark:text-gray-400">
          Unlike other platforms, the Hanzo Commerce Framework allows you to easily
          customize and extend the behavior of your commerce platform to always
          fit your business needs.
        </p>
      </div>
    </div>
  )
}

export default HomepageFrameworkSection
