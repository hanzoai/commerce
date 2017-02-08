package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"

	. "hanzo.io/models/types/analytics"
)

var Organization = New("organization", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	// Such tees owner &operator
	user := User(c).(*user.User)

	// Our fake T-shirt company
	org := organization.New(db)
	org.Name = "suchtees"
	org.GetOrCreate("Name=", org.Name)

	// Our fake T-shirt company continued
	org.FullName = "Such Tees, Inc."
	org.Owners = []string{user.Id()}
	org.Website = "http://suchtees.com"
	org.SecretKey = []byte("prettyprettyteesplease")

	// Saved stripe tokens
	org.Stripe.Test.UserId = "acct_14lSsRCSRlllXCwP"
	org.Stripe.Test.AccessToken = "sk_test_REDACTED"
	org.Stripe.Test.PublishableKey = "pk_test_REDACTED"
	org.Stripe.Test.RefreshToken = "rt_6kqLkyTC8IgfJOSlxjECmGaJfLbWyhy2BY3GgXry4tlzm6rZ"

	// You can only have one set of test credentials, so live/test are the same.
	org.Stripe.Live.UserId = org.Stripe.Test.UserId
	org.Stripe.Live.AccessToken = org.Stripe.Test.AccessToken
	org.Stripe.Live.PublishableKey = org.Stripe.Test.PublishableKey
	org.Stripe.Live.RefreshToken = org.Stripe.Test.RefreshToken

	org.Stripe.UserId = org.Stripe.Test.UserId
	org.Stripe.AccessToken = org.Stripe.Test.AccessToken
	org.Stripe.PublishableKey = org.Stripe.Test.PublishableKey
	org.Stripe.RefreshToken = org.Stripe.Test.RefreshToken

	org.Paypal.ConfirmUrl = "http://www.hanzo.io"
	org.Paypal.CancelUrl = "http://www.hanzo.io"

	org.Paypal.Live.Email = "dev@hanzo.ai"
	org.Paypal.Live.SecurityUserId = "dev@hanzo.ai"
	org.Paypal.Live.ApplicationId = "APP-80W284485P519543T"
	org.Paypal.Live.SecurityPassword = ""
	org.Paypal.Live.SecuritySignature = ""

	org.Paypal.Test.Email = "dev@hanzo.ai"
	org.Paypal.Test.SecurityUserId = "dev@hanzo.ai"
	org.Paypal.Test.ApplicationId = "APP-80W284485P519543T"
	org.Paypal.Test.SecurityPassword = ""
	org.Paypal.Test.SecuritySignature = ""

	// Add default analytics config
	integrations := []Integration{
		Integration{
			Type: "facebook-audiences",
			Id:   "920910517982389",
		},
		Integration{
			Type:  "facebook-conversions",
			Id:    "6025763568614",
			Event: "Sign-up",
		},
		Integration{
			Type: "google-analytics",
			Id:   "UA-65099214-1",
		},
		Integration{
			Type:  "google-adwords",
			Id:    "945491661",
			Event: "Sign-up",
		},
	}
	org.Analytics = Analytics{integrations}

	// Save org into default namespace
	org.MustUpdate()

	// Add org to user and also save
	user.Organizations = []string{org.Id()}
	user.MustUpdate()
	return org
})
