package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models/organization"
	"crowdstart.io/models/user"
	"crowdstart.io/util/log"
)

var Organization = New("organization", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	// Such tees owner &operator
	user := User(c).(*user.User)

	// Our fake T-shirt company
	org := organization.New(db)
	org.Name = "suchtees"
	org.GetOrCreate("Name=", org.Name)

	org.FullName = "Such Tees, Inc."
	org.Owners = []string{user.Id()}
	org.Website = "http://suchtees.com"
	org.SecretKey = []byte("prettyprettyteesplease")

	// Saved stripe tokens
	org.Stripe.Test.UserId = "acct_14lSsRCSRlllXCwP"
	org.Stripe.Test.AccessToken = "sk_test_pApnjF5lBAgmO14BCzXGoGOB"
	org.Stripe.Test.PublishableKey = "pk_test_IDltLDxulChR8tw9rb87JvJk"
	org.Stripe.Test.RefreshToken = "rt_5ySFSL7IYMd6jL8p7gijCZf8jURx0z3Cg02sGGAo3PaQAytq"

	// You can only have one set of test credentials, so live/test are the same.
	org.Stripe.Live.UserId = org.Stripe.Test.UserId
	org.Stripe.Live.AccessToken = org.Stripe.Test.AccessToken
	org.Stripe.Live.PublishableKey = org.Stripe.Test.PublishableKey
	org.Stripe.Live.RefreshToken = org.Stripe.Test.RefreshToken

	org.Stripe.UserId = org.Stripe.Test.UserId
	org.Stripe.AccessToken = org.Stripe.Test.AccessToken
	org.Stripe.PublishableKey = org.Stripe.Test.PublishableKey
	org.Stripe.RefreshToken = org.Stripe.Test.RefreshToken

	// Add default access tokens
	org.AddDefaultTokens()
	log.Debug("Adding tokens: %v", org.Tokens)

	// Save org into default namespace
	org.MustPut()

	// Add org to user and also save
	user.Organizations = []string{org.Id()}
	user.MustPut()
	return org
})
