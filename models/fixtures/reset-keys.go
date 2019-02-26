package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/organization"
)

var _ = New("reset-keys", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "sec-demo"
	org.GetOrCreate("Name=", org.Name)

	org.AddDefaultTokens()
	org.MustUpdate()

	return org
})
