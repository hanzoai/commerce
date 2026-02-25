import { HttpTypes } from "@hanzo/commerce-types"
import { Container, Heading } from "@hanzo/commerce-ui"

import { PencilSquare } from "@hanzo/commerce-icons"
import { useTranslation } from "react-i18next"
import { ActionMenu } from "../../../../../components/common/action-menu"
import { DateRangeDisplay } from "../../../../../components/common/date-range-display"

type CampaignConfigurationSectionProps = {
  campaign: HttpTypes.AdminCampaign
}

export const CampaignConfigurationSection = ({
  campaign,
}: CampaignConfigurationSectionProps) => {
  const { t } = useTranslation()

  return (
    <Container className="flex flex-col gap-y-4">
      <div className="flex items-center justify-between">
        <Heading level="h2">{t("campaigns.configuration.header")}</Heading>
        <ActionMenu
          groups={[
            {
              actions: [
                {
                  label: t("actions.edit"),
                  icon: <PencilSquare />,
                  to: "configuration",
                },
              ],
            },
          ]}
        />
      </div>
      <DateRangeDisplay
        startsAt={campaign.starts_at}
        endsAt={campaign.ends_at}
        showTime
      />
    </Container>
  )
}
