package fixtures

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/auth/password"
	"crowdstart.com/datastore"
	"crowdstart.com/models/namespace"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/store"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/user"
	"crowdstart.com/thirdparty/mailchimp"
	"crowdstart.com/util/token"
)

var Ludela = New("ludela", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	// Create user
	usr := user.New(db)
	usr.Email = "jamie@ludela.com"
	usr.GetOrCreate("Email=", usr.Email)
	usr.FirstName = "Jamie"
	usr.LastName = "Bianchini"
	usr.PasswordHash, _ = password.Hash("1Ludela23")

	// Create organization
	org := organization.New(db)
	org.Name = "ludela"
	org.GetOrCreate("Name=", org.Name)
	org.SetKey("V9OT22mI0a")

	// Set organization on user
	usr.Organizations = []string{org.Id()}

	org.FullName = "Ludela Inc"
	org.Owners = []string{usr.Id()}
	org.Website = "https://ludela.com"
	org.SecretKey = []byte("EU8E011iX2Bp5lv481N2STd1d999cU58")
	org.AddDefaultTokens()
	org.Fee = 0.05

	// Email configuration
	org.Mailchimp.APIKey = ""
	org.Mailchimp.ListId = "262350bdb1"
	org.Mandrill.APIKey = "40gP4DdLRLHo1QX_A8mfHw"

	// Enable accounts by default
	org.SignUpOptions.AccountsEnabledByDefault = true
	org.SignUpOptions.NoNameRequired = true
	org.SignUpOptions.NoPasswordRequired = true
	org.SignUpOptions.TwoStageEnabled = true
	org.SignUpOptions.ImmediateLogin = true

	// API Tokens
	org.Tokens = []token.Token{
		token.Token{
			EntityId:    "V9OT22mI0a",
			Id:          "XodGra0dirg",
			IssuedAt:    time.Now(),
			Name:        "live-secret-key",
			Permissions: 20,
			Secret:      []byte("EU8E011iX2Bp5lv481N2STd1d999cU58"),
		},
		token.Token{
			EntityId:    "V9OT22mI0a",
			Id:          "z2ZCUCxkfhE",
			IssuedAt:    time.Now(),
			Name:        "live-published-key",
			Permissions: 4503617075675172,
			Secret:      []byte("EU8E011iX2Bp5lv481N2STd1d999cU58"),
		},
		token.Token{
			EntityId:    "V9OT22mI0a",
			Id:          "hwsF9-4etJ4",
			IssuedAt:    time.Now(),
			Name:        "test-secret-key",
			Permissions: 24,
			Secret:      []byte("EU8E011iX2Bp5lv481N2STd1d999cU58"),
		},
		token.Token{
			EntityId:    "V9OT22mI0a",
			Id:          "GjpBDnTuDUk",
			IssuedAt:    time.Now(),
			Name:        "test-published-key",
			Permissions: 4503617075675176,
			Secret:      []byte("EU8E011iX2Bp5lv481N2STd1d999cU58"),
		},
	}

	org.Email.Defaults.Enabled = true
	org.Email.Defaults.FromName = "LuDela"
	org.Email.Defaults.FromEmail = "hi@ludela.com"

	org.Email.OrderConfirmation.Subject = "LuDela Order Confirmation"
	org.Email.OrderConfirmation.Enabled = true

	org.Email.User.PasswordReset.Subject = "Reset your LuDela password"
	org.Email.User.PasswordReset.Enabled = true

	// org.Email.User.EmailConfirmation.Subject = ""
	org.Email.User.EmailConfirmation.Enabled = false

	org.Email.User.EmailConfirmed.Subject = "Complete LuDela Registration"
	org.Email.User.EmailConfirmed.Enabled = true

	// Save org into default namespace
	org.Put()

	// Save namespace so we can decode keys for this organization later
	ns := namespace.New(db)
	ns.Name = org.Name
	ns.GetOrCreate("Name=", ns.Name)
	ns.IntId = org.Key().IntID()
	ns.Update()

	// Create namespaced context
	nsdb := datastore.New(org.Namespaced(db.Context))

	// Create new store
	stor := store.New(nsdb)
	stor.Name = "development"
	stor.GetOrCreate("Name=", stor.Name)
	stor.Prefix = "/"
	stor.Currency = currency.USD
	stor.Mailchimp.APIKey = ""
	stor.Mailchimp.ListId = "262350bdb1"
	stor.MustUpdate()

	// Set default store on org
	org.DefaultStore = stor.Id()

	// // Create smart candle
	// prod := product.New(nsdb)
	// prod.Slug = "candle"
	// prod.GetOrCreate("Slug=", prod.Slug)
	// prod.SetKey("9V84cGS9VK")
	// prod.Name = "LuDela Candle"
	// prod.Description = "1 Smart Candle"
	// prod.Price = currency.Cents(19999)
	// prod.Inventory = 9000
	// prod.Preorder = true
	// prod.Hidden = false

	// Save entities
	usr.MustUpdate()
	org.MustUpdate()
	stor.MustUpdate()
	// prod.MustUpdate()

	// Create corresponding Mailchimp entities
	client := mailchimp.New(db.Context, org.Mailchimp.APIKey)
	client.CreateStore(stor)
	// client.CreateProduct(stor.Id(), prod)

	return org
})
