package fixtures

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.io/middleware"
	"crowdstart.io/models2/campaign"
	"crowdstart.io/util/category"
)

const Month = time.Hour * 24 * 30

func Campaign(c *gin.Context) *campaign.Campaign {
	db := getDb(c)
	org := middleware.GetOrg(c)

	campaign := campaign.New(db)
	campaign.OrganizationId = org.Id()
	campaign.Approved = true
	campaign.Enabled = true
	campaign.Category = category.Arts
	campaign.Title = "Such shirt!"
	campaign.Description = "Shirt that bring much happiness."

	campaign.MustPut()
	return campaign
}
