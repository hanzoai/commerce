package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/organization"
)

func Organization(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	// Such tees owner &operator
	user := User(c)

	// Our fake T-shirt company
	org := organization.New(db)
	org.Name = "suchtees"
	org.GetOrCreate("Name=", org.Name)

	org.FullName = "Such Tees, Inc."
	org.Owners = []string{user.Id()}
	org.Website = "http://suchtees.com"
	org.SecretKey = []byte("prettyprettyteesplease")
	org.Stripe.AccessToken = "sk_test_dmur0QtOCRZptNfRNV0uNexi"
	org.Stripe.PublishableKey = "pk_test_VbexM7S8lSitV3xCGLm2kbIx"
	org.AddDefaultTokens()

	// Save org into default namespace
	org.MustPut()

	// Add user as owner
	user.Organizations = []string{org.Id()}
	user.MustPut()
	return org
}
