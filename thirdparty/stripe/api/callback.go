package api

import (
	"strings"

	"appengine"

	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/affiliate"
	"crowdstart.com/models/organization"
	"crowdstart.com/thirdparty/stripe/connect"
	"crowdstart.com/util/log"
)

// Handle stripe Connect callbacks
func Callback(c *gin.Context) {
	url := c.Request.URL
	state := url.Query().Get("state")

	if state != "" && state != "movetoserver" {
		// Affiliate callback
		affiliateCallback(c)
	} else {
		// Redirect to platform
		c.Redirect(302, config.UrlFor("platform", "/stripe/callback")+"?"+url.RawQuery)
	}
}

// Connect callback for affiliates
func affiliateCallback(c *gin.Context) {
	url := c.Request.URL
	code := url.Query().Get("code")
	state := url.Query().Get("state")
	errStr := url.Query().Get("error")

	// Get organization and affiliate id
	parts := strings.Split(state, ":")
	orgId := parts[0]
	affId := parts[1]

	// Fetch organization
	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)
	org := organization.New(db)

	// Failed to get back authorization code from Stripe
	if err := org.GetById(orgId); err != nil {
		log.Error("Failed fetch organization: %v", err, c)
		c.Redirect(302, org.Affiliate.ErrorUrl)
		return
	}

	// Fetch affiliate
	nsctx, _ := appengine.Namespace(ctx, org.Name)
	db = datastore.New(nsctx)
	aff := affiliate.New(db)
	aff.GetById(affId)

	// Failed to get back authorization code from Stripe
	if errStr != "" {
		log.Error("Failed to get authorization code from Stripe during Stripe Connect: %v", errStr, c)
		c.Redirect(302, org.Affiliate.ErrorUrl)
		return
	}

	// Get live and test tokens
	token, testToken, err := connect.GetTokens(ctx, code)
	if err != nil {
		log.Error("There was an error with Stripe Connect: %v", err, c)
		c.Redirect(302, org.Affiliate.ErrorUrl)
		return
	}

	// Update affiliate
	aff.Connected = true
	aff.Stripe.UserId = token.UserId
	aff.Stripe.AccessToken = token.AccessToken
	aff.Stripe.PublishableKey = token.PublishableKey
	aff.Stripe.RefreshToken = token.RefreshToken

	// Save live/test tokens
	aff.Stripe.Live = *token
	aff.Stripe.Test = *testToken

	// Save to datastore
	if err := aff.Put(); err != nil {
		log.Error("There was saving tokens to datastore: %v", err, c)
		c.Redirect(302, org.Affiliate.ErrorUrl)
		return
	}

	// Success
	c.Redirect(302, org.Affiliate.SuccessUrl)
}
