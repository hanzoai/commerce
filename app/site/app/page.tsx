import clsx from "clsx"
import Providers from "../providers"
import HomepageTopSection from "../components/Homepage/TopSection"
import HomepageSectionsSeparator from "../components/Homepage/SectionsSeparator"
import HomepageLinksSection from "../components/Homepage/LinksSection"
import HomepageFrameworkSection from "../components/Homepage/FrameworkSection"
import HomepageCodeTabs from "../components/Homepage/CodeTabs"
import HomepageRecipesSection from "../components/Homepage/RecipesSection"
import HomepageCommerceModulesSection from "../components/Homepage/CommerceModulesSection"
import HomepageFooter from "../components/Homepage/Footer"

const Homepage = () => {
  return (
    <body
      className={clsx(
        "font-sans text-base w-full",
        "text-gray-900 dark:text-gray-100",
        "h-screen overflow-hidden"
      )}
    >
      <Providers>
        <div
          className={clsx(
            "bg-white dark:bg-gray-950",
            "h-full w-full",
            "overflow-y-scroll overflow-x-hidden"
          )}
          id="main"
        >
          <nav className="border-b border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900 px-4 py-3 flex items-center justify-between">
            <a href="/" className="flex items-center gap-2">
              <span className="text-xl font-bold">Hanzo Commerce</span>
            </a>
            <div className="flex items-center gap-4 text-sm">
              <a href="/learn" className="hover:text-blue-600 dark:hover:text-blue-400">Docs</a>
              <a href="https://admin.commerce.hanzo.ai" className="hover:text-blue-600 dark:hover:text-blue-400">Dashboard</a>
              <a href="https://github.com/hanzoai/commerce" className="hover:text-blue-600 dark:hover:text-blue-400">GitHub</a>
            </div>
          </nav>
          <div
            className={clsx(
              "xl:mx-auto xl:max-w-[1026px] w-full",
              "flex flex-col justify-center items-start",
              "xl:border-x xl:border-gray-200 dark:xl:border-gray-800"
            )}
          >
            <HomepageSectionsSeparator />
            <HomepageTopSection />
            <HomepageSectionsSeparator />
            <HomepageLinksSection />
            <HomepageSectionsSeparator />
            <HomepageFrameworkSection />
            <HomepageCodeTabs />
            <HomepageSectionsSeparator />
            <HomepageRecipesSection />
            <HomepageSectionsSeparator />
            <HomepageCommerceModulesSection />
            <HomepageSectionsSeparator />
            <HomepageFooter />
          </div>
        </div>
      </Providers>
    </body>
  )
}

export default Homepage
