package fixtures

import (
	// "time"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/types/email"
	"hanzo.io/types/email/provider"
	"hanzo.io/types/integration"
)

var _ = New("damon-email", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "damon"
	org.GetOrCreate("Name=", org.Name)

	// Email configuration
	org.Mandrill.APIKey = ""

	org.Email.Enabled = true
	org.Email.Defaults.From = email.Email{
		Name:    "Damon Motorcycles",
		Address: "hi@damonmotorcycles.com",
	}
	org.Email.Defaults.ProviderId = string(provider.Mandrill)
	org.Email.Order.Confirmation = email.Setting{
		Enabled:    true,
		TemplateId: "order-confirmed",
	}

	if mandrills := org.Integrations.FilterByType(integration.MandrillType); len(mandrills) == 0 {
		m := integration.Integration{
			Type:     integration.MandrillType,
			Enabled:  true,
			Mandrill: org.Mandrill,
		}
		org.Integrations.MustAppend(&m)
	}

	// Save org into default namespace
	org.MustUpdate()

	return org
})
