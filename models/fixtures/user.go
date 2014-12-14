package fixtures

import (
	"time"

	"appengine"
	"appengine/delay"

	"code.google.com/p/go.crypto/bcrypt"

	"crowdstart.io/datastore"
	. "crowdstart.io/models"
)

var testUsers = delay.Func("fixtures-test-users", func(c appengine.Context) {
	db := datastore.New(c)

	// Add default test user
	pwhash, _ := bcrypt.GenerateFromPassword([]byte("password"), 12)

	db.PutKey("user", "test@test.com", &User{
		Id:           "test@test.com",
		FirstName:    "Test",
		LastName:     "User",
		Email:        "test@test.com",
		Phone:        "(123) 456-7890",
		PasswordHash: pwhash,
	})

	// Create token
	token := new(InviteToken)
	token.Id = "test-token"
	token.Email = "test@test.com"
	db.PutKey("invite-token", "test-token", token)

	// Save contribution
	contribution := Contribution{
		Id:            "test",
		Perk:          Perks["2267279"],
		Status:        "Unfulfilled",
		FundingDate:   "1983-06-30",
		PaymentMethod: "PayPal",
		Email:         "test@test.com",
	}
	db.PutKey("contribution", "test", &contribution)

	order := Order{
		Id:        "test-order",
		CreatedAt: time.Now(),
		Email:     "test@test.com",
		Preorder:  true,
	}
	db.PutKey("order", "test@test.com", &order)
})

var skullyUser = delay.Func("fixtures-skully-user", func(c appengine.Context) {
	db := datastore.New(c)

	// Add SKULLY user
	pwhash, _ := bcrypt.GenerateFromPassword([]byte("Victory1!"), 12)

	db.PutKey("user", "dev@hanzo.ai", &User{
		Id:           "dev@hanzo.ai",
		FirstName:    "Mitchell",
		LastName:     "Weller",
		Email:        "dev@hanzo.ai",
		Phone:        "(123) 456-7890",
		PasswordHash: pwhash,
	})
})

var skullyCampaign = delay.Func("fixtures-skully-campaign", func(c appengine.Context) {
	db := datastore.New(c)

	// Default Campaign (SKULLY)
	campaign := Campaign{
		Id:    "dev@hanzo.ai",
		Title: "SKULLY AR-1",
	}

	// Hardcode stripe test credentials
	if appengine.IsDevAppServer() {
		campaign.Stripe.AccessToken = "sk_test_eyQyQYZlwLcKxM9LoxLKg61y"
		campaign.Stripe.PublishableKey = "pk_test_IkyRgPrDxa5SRvEP1XKpJann"
		campaign.Stripe.RefreshToken = "rt_5E65oPVEYWwIAqBWpW64RfefExYPVAvt4Pu9YeEBPJn9AECa"
		campaign.Stripe.UserId = "acct_14lSsRCSRlllXCwP"
	}

	db.PutKey("campaign", "dev@hanzo.ai", &campaign)
})
