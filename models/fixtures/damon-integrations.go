package fixtures

import (
	// "time"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/types/integration"
)

var _ = New("damon-integrations", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "damon"
	org.GetOrCreate("Name=", org.Name)

	wpr := &integration.Integration{
		Type:    integration.WoopraType,
		Enabled: true,
		Woopra: integration.Woopra{
			Domain: "damon.com",
		},
	}

	if len(org.Integrations.FilterByType(wpr.Type)) == 0 {
		org.Integrations = org.Integrations.MustAppend(wpr)
	}

	// m := integration.Integration{
	// 	Type:     integration.MandrillType,
	// 	Enabled:  true,
	// 	Mandrill: org.Mandrill,
	// }
	// org.Integrations.MustAppend(&m)

	// Save org into default namespace
	org.MustUpdate()

	return org
})
