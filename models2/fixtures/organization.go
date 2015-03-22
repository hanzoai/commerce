package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/organization"
)

func Organization(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	// Owner for this organization
	user := User(c)

	// Our fake T-shirt company
	org := organization.New(db)
	org.Name = "suchtees"
	org.GetOrCreate("Name=", org.Name)

	org.FullName = "Such Tees, Inc."
	org.OwnerId = user.Id()
	org.Website = "http://suchtees.com"
	org.SecretKey = []byte("prettyprettyteesplease")
	org.Stripe.AccessToken = "sk_test_dmur0QtOCRZptNfRNV0uNexi"
	org.Stripe.PublishableKey = "pk_test_VbexM7S8lSitV3xCGLm2kbIx"
	org.Put()

	// Save into org's namespace
	ctx := org.Namespace(c)
	user.SetContext(ctx)
	org.SetContext(ctx)

	org.Put()
	user.Put()

	return org
}
