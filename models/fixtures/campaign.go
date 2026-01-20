package fixtures

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/models/campaign"
	"github.com/hanzoai/commerce/util/category"
)

var Campaign = New("campaign", func(c *gin.Context) *campaign.Campaign {
	db := getNamespaceDb(c)
	org := Organization(c)

	campaign := campaign.New(db)
	campaign.Slug = "some-campaign"
	campaign.GetOrCreate("Slug=", campaign.Slug)
	campaign.Parent = org.Key()
	campaign.OrganizationId = org.Id()
	campaign.Approved = true
	campaign.Enabled = true
	campaign.Category = category.Arts
	campaign.Title = "Such shirt!"
	campaign.Description = "Shirt that bring much happiness."

	campaign.MustPut()
	return campaign
})
