package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/namespace"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/user"
	"crowdstart.com/util/log"

	. "crowdstart.com/models/types/analytics"
)

var Organization = New("organization", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	// Such tees owner &operator
	usr := User(c).(*user.User)

	// Our fake T-shirt company
	org := organization.New(db)
	org.Name = "suchtees"
	org.GetOrCreate("Name=", org.Name)

	org.FullName = "Such Tees, Inc."
	org.Owners = []string{usr.Id()}
	org.Website = "http://suchtees.com"
	org.SecretKey = []byte("prettyprettyteesplease")

	// Saved stripe tokens
	org.Stripe.Test.UserId = "acct_16fNBDH4ZOGOmFfW"
	org.Stripe.Test.AccessToken = "sk_test_RnnTXycI4vLympetwb66jTab"
	org.Stripe.Test.PublishableKey = "pk_test_1Y8PTDLIWERNUYcpg8tglNBY"
	org.Stripe.Test.RefreshToken = "rt_9MArkOe9fEf4bDRstgha9Ma6r6W5JM5c3LWlWFBRwv9iA2qi"

	// You can only have one set of test credentials, so live/test are the same.
	org.Stripe.Live.UserId = org.Stripe.Test.UserId
	org.Stripe.Live.AccessToken = org.Stripe.Test.AccessToken
	org.Stripe.Live.PublishableKey = org.Stripe.Test.PublishableKey
	org.Stripe.Live.RefreshToken = org.Stripe.Test.RefreshToken

	org.Stripe.UserId = org.Stripe.Test.UserId
	org.Stripe.AccessToken = org.Stripe.Test.AccessToken
	org.Stripe.PublishableKey = org.Stripe.Test.PublishableKey
	org.Stripe.RefreshToken = org.Stripe.Test.RefreshToken

	org.Paypal.ConfirmUrl = "http://www.crowdstart.com"
	org.Paypal.CancelUrl = "http://www.crowdstart.com"

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

	// Add default access tokens
	org.AddDefaultTokens()
	log.Debug("Adding tokens: %v", org.Tokens)

	// Add default analytics config
	integrations := []Integration{
		Integration{
			Type: "facebook-pixel",
			Id:   "920910517982389",
		},
		Integration{
			Type: "google-analytics",
			Id:   "UA-65099214-1",
		},
	}
	org.Analytics = Analytics{integrations}

	// Save org into default namespace
	org.MustPut()

	// Save namespace so we can decode keys for this organization later
	ns := namespace.New(db)
	ns.Name = org.Name
	ns.IntId = org.Key().IntID()
	err := ns.Put()
	if err != nil {
		log.Warn("Failed to put namespace: %v", err)
	}

	// Add org to user and also save
	usr.Organizations = []string{org.Id()}
	usr.MustPut()
	return org
})
