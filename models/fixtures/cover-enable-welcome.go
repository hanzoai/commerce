package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/organization"
)

var CoverEnableWelcome = New("cover-enable-welcome", func(c *context.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.MustGetById("cover")
	org.Email.Subscriber.Welcome.Enabled = true
	return org
})
