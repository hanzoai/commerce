package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/campaign"
	"crowdstart.com/util/category"
)

var Campaign = New("campaign", func(c *gin.Context) *campaign.Campaign {
	db := getNamespaceDb(c)
	org := Organization(c)

	campaign := campaign.New(db)
	campaign.Slug = "some-campaign"
	campaign.GetOrCreate("Slug=", campaign.Slug)
	campaign.Ancestor = org.Key()
	campaign.OrganizationId = org.Id()
	campaign.Approved = true
	campaign.Enabled = true
	campaign.Category = category.Arts
	campaign.Title = "Such shirt!"
	campaign.Description = "Shirt that bring much happiness."

	campaign.MustPut()
	return campaign
})
