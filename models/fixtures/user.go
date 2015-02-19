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
	db.PutKind("invite-token", "test-token", token)

	// Save contribution
	contribution := Contribution{
		Id:            "test",
		Perk:          Perks["2267279"],
		Status:        "Unfulfilled",
		FundingDate:   "1983-06-30",
		PaymentMethod: "PayPal",
		UserId:        user.Id,
	}
	db.PutKind("contribution", "test", &contribution)

	order := Order{
		Id:        "test-order",
		CreatedAt: time.Now(),
		UserId:    user.Id,
		Preorder:  true,
	}
	db.PutKind("order", order.Id, &order)
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
		campaign.Salesforce.AccessToken = "00Do0000000d5HA!ARcAQAC4j9MdFY5T0jElLYZu_W_qn0IUZQVOrVPD6H9yhHvtL4HKpagHnfKptQlIeLyV0ndPuEcn7YjRhWPGYEIuI4osn.GC"
		campaign.Salesforce.RefreshToken = "5Aep861LNDQReieQSK6OvPpwG_C1z9MoX7qJR8huC9h.oOQm.eW2gfv6sfo9AUJgTUNnH4Tx3qBz9XtZGK2j1oS"
		campaign.Salesforce.Id = "ttps://login.salesforce.com/id/00Do0000000d5HAEAY/005o0000001VCsiAAG"
		campaign.Salesforce.IssuedAt = "1419371438825"
		campaign.Salesforce.InstanceUrl = "https://na17.salesforce.com"
		campaign.Salesforce.Signature = "RO086wMIGu1bLlXgjtMtAk4JGSd8k2/yb5tKRGq/No8="
		campaign.Salesforce.DefaultPriceBookId = "01so0000003EAuw"
	}
	db.PutKind("campaign", "dev@hanzo.ai", &campaign)
})
