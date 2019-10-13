package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	// "hanzo.io/models/namespace"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/organization"
	"hanzo.io/models/shippingrates"
	"hanzo.io/models/store"
	"hanzo.io/models/taxrates"
	"hanzo.io/models/types/georate"
	"hanzo.io/models/user"
	"hanzo.io/types/website"

	. "hanzo.io/models/types/analytics"
)

var Organization = New("organization", func(c *gin.Context) *organization.Organization {
	BlockchainNamespace(c)

	db := datastore.New(c)

	// Such tees owner &operator
	usr := User(c).(*user.User)

	// Our fake T-shirt company
	org := organization.New(db)
	org.Name = "suchtees"
	org.SecretKey = []byte("prettyprettyteesplease")
	org.GetOrCreate("Name=", org.Name)

	org.FullName = "Such Tees, Inc."
	org.Owners = []string{usr.Id()}
	org.Websites = []website.Website{website.Website{Type: website.Production, Url: "http://suchtees.com"}}

	// Saved stripe tokens
	org.Stripe.Test.UserId = "acct_16fNBDH4ZOGOmFfW"
	org.Stripe.Test.AccessToken = ""
	org.Stripe.Test.PublishableKey = "pk_test_HHiaCsBYlyfI45xtAvIAsjRe"
	org.Stripe.Test.RefreshToken = ""

	org.AuthorizeNet.Sandbox.LoginId = ""
	org.AuthorizeNet.Sandbox.TransactionKey = ""
	org.AuthorizeNet.Sandbox.Key = "Simon"

	// Ethereum
	org.Ethereum.Address = "0xf2fccc0198fc6b39246bd91272769d46d2f9d43b"
	org.Bitcoin.Address = ""
	org.Bitcoin.TestAddress = "mrPFGX5ViUZk2s8i5soBCkrFVzRwngK8DQ"

	// You can only have one set of test credentials, so live/test are the same.
	org.Stripe.Live.UserId = org.Stripe.Test.UserId
	org.Stripe.Live.AccessToken = org.Stripe.Test.AccessToken
	org.Stripe.Live.PublishableKey = org.Stripe.Test.PublishableKey
	org.Stripe.Live.RefreshToken = org.Stripe.Test.RefreshToken

	org.Stripe.UserId = org.Stripe.Test.UserId
	org.Stripe.AccessToken = org.Stripe.Test.AccessToken
	org.Stripe.PublishableKey = org.Stripe.Test.PublishableKey
	org.Stripe.RefreshToken = org.Stripe.Test.RefreshToken

	org.Paypal.ConfirmUrl = "http://hanzo.io"
	org.Paypal.CancelUrl = "http://hanzo.io"

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

	org.WalletPassphrase = "1234"

	w, _ := org.GetOrCreateWallet(org.Db)
	a1, _ := w.CreateAccount("Test Ethereum", blockchains.EthereumRopstenType, []byte(org.WalletPassphrase))
	a1.Withdrawable = true
	a2, _ := w.CreateAccount("Test Bitcoin", blockchains.BitcoinTestnetType, []byte(org.WalletPassphrase))
	a2.Withdrawable = true
	w.MustUpdate()

	// Add default access tokens
	// org.AddDefaultTokens()
	// log.Debug("Adding tokens: %v", org.Tokens)

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
	org.Analytics = Analytics{Integrations: integrations}

	// Save org into default namespace
	org.MustPut()

	// Retrofit existing thing
	if org.DefaultStore == "" {
		nsdb := datastore.New(org.Namespaced(org.Context()))

		stor := store.New(nsdb)
		stor.GetOrCreate("Name=", "Default")
		stor.Name = "Default"
		stor.Currency = org.Currency
		stor.MustUpdate()

		trs := taxrates.New(nsdb)
		trs.GetOrCreate("StoreId=", stor.Id())
		trs.StoreId = stor.Id()
		trs.MustCreate()

		srs := shippingrates.New(nsdb)
		srs.GetOrCreate("StoreId=", stor.Id())
		srs.StoreId = stor.Id()
		srs.MustCreate()

		org.DefaultStore = stor.Id()
		org.MustUpdate()
	}

	stor, _ := org.GetDefaultStore()

	trs, _ := stor.GetTaxRates()
	trs.GeoRates = []taxrates.GeoRate{
		taxrates.GeoRate{
			GeoRate: georate.New(
				"US",
				"MO",
				"",
				"64108",
				0.08475,
				0,
			),
		},
		taxrates.GeoRate{
			GeoRate: georate.New(
				"US",
				"MO",
				"",
				"",
				0.04225,
				0,
			),
		},
	}

	trs.MustUpdate()

	srs, _ := stor.GetShippingRates()
	srs.GeoRates = []shippingrates.GeoRate{
		shippingrates.GeoRate{
			GeoRate: georate.New(
				"US",
				"",
				"",
				"",
				0,
				499,
			),
		},
		shippingrates.GeoRate{
			GeoRate: georate.New(
				"",
				"",
				"",
				"",
				0,
				999,
			),
		},
	}

	srs.MustUpdate()

	// Save namespace so we can decode keys for this organization later
	// ns := namespace.New(db)
	// ns.Name = org.Name
	// ns.IntId = org.Key().IntID()
	// err := ns.Put()
	// if err != nil {
	// 	log.Warn("Failed to put namespace: %v", err)
	// }

	// Add org to user and also save
	usr.Organizations = []string{org.Id()}
	usr.MustPut()
	return org
})
