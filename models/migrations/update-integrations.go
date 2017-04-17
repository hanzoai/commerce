package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/organization"
	"hanzo.io/models/types/integrations"
	"hanzo.io/util/json"
	// "hanzo.io/util/log"

	ds "hanzo.io/datastore"
)

var _ = New("update-integrations",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "")
		return NoArgs
	},
	func(db *ds.Datastore, org *organization.Organization) {
		for i, an := range org.Analytics.Integrations {
			in := integrations.Integration{}
			switch an.Type {
			case "custom":
				in.Type = integrations.AnalyticsCustomType
			case "facebook-conversions":
				in.Type = integrations.AnalyticsFacebookConversionsType
			case "facebook-pixel":
				in.Type = integrations.AnalyticsFacebookPixelType
			case "google-adwords":
				in.Type = integrations.AnalyticsGoogleAdwordsType
			case "google-analytics":
				in.Type = integrations.AnalyticsGoogleAnalyticsType
			case "heap":
				in.Type = integrations.AnalyticsHeapType
			}

			in.Id = an.IntegrationId
			in.Data = json.EncodeBytes(an)

			org.Integrations.MustUpdate(&in)

			org.Analytics.Integrations[i].IntegrationId = in.Id
		}
		org.Update()
	},
)
