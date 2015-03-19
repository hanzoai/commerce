package admin

import (
	"log"
	"net/http"
	"net/url"
	"strings"

	"appengine/urlfetch"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/thirdparty/stripe/tasks"
	"crowdstart.io/util/json"
	"crowdstart.io/util/template"
)

type stripeToken struct {
	AccessToken          string `json:"access_token"`
	Error                string `json:"error"`
	ErrorDescription     string `json:"error_description"`
	Livemode             bool   `json:"livemode"`
	RefreshToken         string `json:"refresh_token"`
	Scope                string `json:"scope"`
	StripePublishableKey string `json:"stripe_publishable_key"`
	StripeUserId         string `json:"stripe_user_id"`
	TokenType            string `json:"token_type"`
}

/*
Warning
Due to the fact that `CampaignId`s are currently missing in all the orders,
this function assumes that every order is associated with the only campaign (SKULLY).

TODO: Run a migration to set `CampaignId` in all orders.
*/
func StripeSync(c *gin.Context) {
	tasks.RunSynchronizeCharges(c)
	c.String(200, "Synchronising orders")
}

// Admin Payment Connectors
func StripeConnect(c *gin.Context) {
	template.Render(c, "admin/stripe/connect.html",
		"stripe", config.Stripe)
}

// StripeCallback Stripe End Points
func StripeCallback(c *gin.Context) {
	req := c.Request
	code := req.URL.Query().Get("code")
	errStr := req.URL.Query().Get("error")

	// Failed to get back authorization code from Stripe
	if errStr != "" {
		template.Render(c, "admin/stripe/connect.html", "error", errStr)
		return
	}

	ctx := middleware.GetAppEngine(c)
	client := urlfetch.Client(ctx)

	data := url.Values{}
	data.Set("client_secret", config.Stripe.APISecret)
	data.Add("code", code)
	data.Add("grant_type", "authorization_code")

	tokenReq, err := http.NewRequest("POST", "https://connect.stripe.com/oauth/token", strings.NewReader(data.Encode()))
	if err != nil {
		c.Fail(500, err)
		return
	}

	// try to post to OAuth API
	res, err := client.Do(tokenReq)
	defer res.Body.Close()
	if err != nil {
		c.Fail(500, err)
		return
	}

	token := new(stripeToken)
	// try and extract the json struct
	if err := json.Decode(res.Body, token); err != nil {
		c.Fail(500, err)
	}

	// Stripe returned an error
	if token.Error != "" {
		template.Render(c, "admin/stripe/connect.html",
			"stripeError", token.Error,
			"stripe", config.Stripe,
			"salesforce", config.Salesforce)
		return
	}

	// Update the user
	campaign := new(models.Campaign)

	db := datastore.New(ctx)

	// Get email from the session
	email, err := auth.GetEmail(c)
	if err != nil {
		log.Panic("Unable to get email from session: %v", err)
	}

	// Get user instance
	if err := db.GetKind("campaign", email, campaign); err != nil {
		log.Panic("Unable to get campaign from database: %v", err)
	}

	// Update stripe data
	campaign.Stripe.AccessToken = token.AccessToken
	campaign.Stripe.Livemode = token.Livemode
	campaign.Stripe.PublishableKey = token.StripePublishableKey
	campaign.Stripe.RefreshToken = token.RefreshToken
	campaign.Stripe.Scope = token.Scope
	campaign.Stripe.TokenType = token.TokenType
	campaign.Stripe.UserId = token.StripeUserId

	// Update in datastore
	if _, err := db.PutKind("campaign", email, campaign); err != nil {
		log.Panic("Failed to update campaign: %v", err)
	}

	// Success
	template.Render(c, "stripe/success.html", "token", token.AccessToken)
}
