"use client"

// A client component on purpose: it draws with `@hanzo/gui`, and Gui's context
// factories run at module scope. Evaluated in Next's SERVER graph — which is
// what /_not-found does — that scope resolves a React without `createContext`
// and the export dies during page-data collection ("r is not a function").
// Every other page reaches Gui through a "use client" module already.
import Providers from "@/providers"
import { SiteShell, SiteLink } from "@/components/shell"
import { Text, XStack, YStack } from "@hanzo/gui"

const NotFoundPage = () => (
  <body>
    <Providers>
      <SiteShell>
        <YStack items="center" gap="$4" px="$4" py="$12">
          <Text render="h1" fontSize="$11" fontWeight="500" color="$color">
            Page Not Found
          </Text>
          <Text render="p" fontSize="$4" color="$color11">
            The page you were looking for is not available.
          </Text>
          <XStack gap="$3" mt="$3" flexWrap="wrap" justify="center">
            <SiteLink href="/learn">
              <Text px="$4" py="$2" rounded="$3" fontSize="$3" fontWeight="500" bg="$color" color="$background">
                Get Started Docs
              </Text>
            </SiteLink>
            <SiteLink href="https://github.com/hanzoai/commerce/issues">
              <Text px="$4" py="$2" rounded="$3" fontSize="$3" fontWeight="500" borderWidth={1} borderColor="$borderColor" color="$color">
                Report Issue
              </Text>
            </SiteLink>
          </XStack>
        </YStack>
      </SiteShell>
    </Providers>
  </body>
)

export default NotFoundPage
