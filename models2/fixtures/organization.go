package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/user"
)

func Organization(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	// Owner for this organization
	user := user.New(db)
	user.Email = "dev@hanzo.ai"
	user.GetOrCreate("Email=", user.Email)

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
	org.AddDefaultTokens()

	// Save org into default namespace
	org.Put()

	// ..and also save org/user into org's namespace
	ctx := org.Namespace(c)
	user.SetContext(ctx)
	org.SetContext(ctx)
	org.Put()
	user.Put()

	return org
}
