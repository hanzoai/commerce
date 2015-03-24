package admin

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"appengine/urlfetch"

	"crowdstart.io/config"
	"crowdstart.io/middleware"
	"crowdstart.io/thirdparty/stripe/tasks"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
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
	template.Render(c, "admin/stripe/connect.html")
}

// StripeCallback Stripe End Points
func StripeCallback(c *gin.Context) {
	req := c.Request
	code := req.URL.Query().Get("code")
	errStr := req.URL.Query().Get("error")

	// Failed to get back authorization code from Stripe
	if errStr != "" {
		log.Error("Failed to get authorization code from Stripe during Stripe Connect: %v", errStr, c)
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

	// try and extract the json struct
	token := new(stripeToken)
	if err := json.Decode(res.Body, token); err != nil {
		c.Fail(500, err)
		return
	}

	// Stripe returned an error
	if token.Error != "" {
		log.Error("There was an error with Stripe Connect: %v", token.Error, c)
		template.Render(c, "admin/stripe/connect.html", "stripeError", token.Error)
		return
	}

	org := middleware.GetOrganization(c)

	// Update stripe data
	org.Stripe.AccessToken = token.AccessToken
	org.Stripe.Livemode = token.Livemode
	org.Stripe.PublishableKey = token.StripePublishableKey
	org.Stripe.RefreshToken = token.RefreshToken
	org.Stripe.Scope = token.Scope
	org.Stripe.TokenType = token.TokenType
	org.Stripe.UserId = token.StripeUserId

	// Update in datastore
	if err := org.Put(); err != nil {
		c.Fail(500, err)
		return
	}

	// Success
	template.Render(c, "admin/stripe/connect.html")
}
