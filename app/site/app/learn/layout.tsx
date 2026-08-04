import Providers from "@/providers"
import { SiteShell } from "@/components/shell"

export default function LearnLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <body>
      <Providers>
        <SiteShell stickyNav prose>
          {children}
        </SiteShell>
      </Providers>
    </body>
  )
}
