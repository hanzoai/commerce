package fixtures

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/auth/password"
	"crowdstart.com/datastore"
	"crowdstart.com/models/namespace"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/user"
	"crowdstart.com/util/log"
	"crowdstart.com/util/token"
)

var Ludela = New("ludela", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "ludela"
	org.GetOrCreate("Name=", org.Name)
	org.SetKey("V9OT22mI0a")

	u := user.New(db)
	u.Email = "jamie@ludela.com"
	u.GetOrCreate("Email=", u.Email)
	u.FirstName = "Jamie"
	u.LastName = ""
	u.Organizations = []string{org.Id()}
	u.PasswordHash, _ = password.Hash("1Ludela23")
	u.Put()

	org.FullName = "Ludela Inc"
	org.Owners = []string{u.Id()}
	org.Website = "http://www.ludela.com"
	org.SecretKey = []byte("EU8E011iX2Bp5lv481N2STd1d999cU58")
	org.AddDefaultTokens()
	org.Fee = 0.05

	// Email configuration
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
	org.Email.Defaults.FromName = "LUDELA"
	org.Email.Defaults.FromEmail = "hi@ludela.com"

	org.Email.OrderConfirmation.Subject = "LUDELA Order Confirmation"
	org.Email.OrderConfirmation.Enabled = true

	org.Email.User.PasswordReset.Subject = "Reset your LUDELA password"
	org.Email.User.PasswordReset.Enabled = true

	// org.Email.User.EmailConfirmation.Subject = ""
	org.Email.User.EmailConfirmation.Enabled = false

	org.Email.User.EmailConfirmed.Subject = "Complete LUDELA Registration"
	org.Email.User.EmailConfirmed.Enabled = true

	// Save org into default namespace
	org.Put()

	// Save namespace so we can decode keys for this organization later
	ns := namespace.New(db)
	ns.Name = org.Name
	ns.IntId = org.Key().IntID()
	err := ns.Put()
	if err != nil {
		log.Warn("Failed to put namespace: %v", err)
	}

	return org
})
