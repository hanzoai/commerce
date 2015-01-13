package fixtures

import (
	"time"

	"appengine"
	"appengine/delay"

	"code.google.com/p/go.crypto/bcrypt"

	"crowdstart.io/datastore"
	. "crowdstart.io/models"

	"crowdstart.io/util/queries"
)

var testUsers = delay.Func("fixtures-test-users", func(c appengine.Context) {
	db := datastore.New(c)
	q := queries.New(c)

	// Add default test user
	pwhash, _ := bcrypt.GenerateFromPassword([]byte("password"), 12)

	user := &User{
		FirstName:    "Test",
		LastName:     "User",
		Email:        "test@test.com",
		Phone:        "(123) 456-7890",
		PasswordHash: pwhash,
	}
	q.UpsertUser(user)

	// Create token
	token := new(Token)
	token.Id = "test-token"
	token.UserId = user.Id
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
	q := queries.New(c)

	// Add SKULLY user
	pwhash, _ := bcrypt.GenerateFromPassword([]byte("Victory1!"), 12)

	q.UpsertUser(&User{
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
		campaign.Stripe.AccessToken = ""
		campaign.Stripe.PublishableKey = "pk_test_ucSTeAAtkSXVEg713ir40UhX"
		campaign.Stripe.RefreshToken = "rt_5E65oPVEYWwIAqBWpW64RfefExYPVAvt4Pu9YeEBPJn9AECa"
		campaign.Stripe.UserId = "acct_14lSsRCSRlllXCwP"

		// And sales force test credentials
		campaign.Salesforce.AccessToken = "00DU0000000MGvt!AREAQGYKLfUdd85R8MyNpSWElcacGbL1d7.Z1ZmXfswfqbLY2q82CArizIcGgk_uqhLr43vDuK_.cp28IcdAnkGA_CiIesra"
		campaign.Salesforce.RefreshToken = "5Aep861ikNsOLQGnbp74xiVo8YsSB.C3pr13Ap4bZm4gkEn0F7rF2X3J49AMiNBbqmKA0rqQgNrl8kuTNEnEhlK"
		campaign.Salesforce.Id = "https://login.salesforce.com/id/00DU0000000MGvtMAG/005U0000003d6VyIAI"
		campaign.Salesforce.IssuedAt = "1419371438825"
		campaign.Salesforce.InstanceUrl = "https://na12.salesforce.com"
		campaign.Salesforce.Signature = "RO086wMIGu1bLlXgjtMtAk4JGSd8k2/yb5tKRGq/No8="
	}
	db.PutKey("campaign", "dev@hanzo.ai", &campaign)
})
