package fixtures

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
)

var _ = New("cover-enable-welcome", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.MustGetById("cover")
	org.Email.Subscriber.Welcome.Enabled = true
	return org
})
