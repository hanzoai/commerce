package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models2/campaign"
	"crowdstart.io/util/category"
)

func Campaign(c *gin.Context) *campaign.Campaign {
	org := getOrg(c)
	db := getDb(c)

	campaign := campaign.New(db)
	campaign.Parent = org.Key()
	campaign.OrganizationId = org.Id()
	campaign.Approved = true
	campaign.Enabled = true
	campaign.Category = category.Arts
	campaign.Title = "Such shirt!"
	campaign.Description = "Shirt that bring much happiness."

	campaign.MustPut()
	return campaign
}
