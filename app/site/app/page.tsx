import Providers from "@/providers"
import { SiteShell } from "@/components/shell"
import Homepage from "@/components/homepage"

const Page = () => (
  <body>
    <Providers>
      <SiteShell>
        <Homepage />
      </SiteShell>
    </Providers>
  </body>
)

export default Page
