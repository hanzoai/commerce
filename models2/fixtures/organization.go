package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/organization"
	"crowdstart.io/util/log"
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

	// Saved stripe tokens
	org.Stripe.Live.UserId = "acct_14lSsRCSRlllXCwP"
	org.Stripe.Live.AccessToken = "sk_test_dmur0QtOCRZptNfRNV0uNexi"
	org.Stripe.Live.PublishableKey = "pk_test_VbexM7S8lSitV3xCGLm2kbIx"
	org.Stripe.Live.RefreshToken = "rt_5uU4oIaJ9irUxH5dljX0vb2upWBoUVQwUAfuAdUW7mNVUurV"

	org.Stripe.Test.UserId = "acct_14lSsRCSRlllXCwP"
	org.Stripe.Test.AccessToken = "sk_test_5zFDvQKcEtxRrEacwWONryPJ"
	org.Stripe.Test.PublishableKey = "pk_test_3EKUm4ssdKZobyO18fd5AShm"
	org.Stripe.Test.RefreshToken = "rt_5uU4oIaJ9irUxH5dljX0vb2upWBoUVQwUAfuAdUW7mNVUurV"

	// Default to live
	org.Stripe.UserId = org.Stripe.Live.UserId
	org.Stripe.AccessToken = org.Stripe.AccessToken
	org.Stripe.PublishableKey = org.Stripe.PublishableKey
	org.Stripe.RefreshToken = org.Stripe.RefreshToken

	org.AddDefaultTokens()
	log.Debug("Adding tokens: %v", org.Tokens)

	// Save org into default namespace
	org.MustPut()

	// Add user as owner
	user.Organizations = []string{org.Id()}
	user.MustPut()
	return org
}
