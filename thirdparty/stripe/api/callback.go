package api

import (
	"fmt"
	"strings"

	"github.com/hanzoai/commerce/util/rand"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/affiliate"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/thirdparty/kms"
	"github.com/hanzoai/commerce/thirdparty/stripe/connect"
	"github.com/hanzoai/commerce/types/integration"
)

// Handle stripe Connect callbacks
func Callback(c *gin.Context) {
	url := c.Request.URL
	state := url.Query().Get("state")

	if strings.Contains(state, ":") {
		// Affiliate callback
		affiliateCallback(c)
	} else {
		organizationCallback(c)
	}
}

func organizationCallback(c *gin.Context) {
	url := c.Request.URL
	ctx := middleware.GetContext(c)
	db := datastore.New(ctx)
	org := organization.New(db)

	code := url.Query().Get("code")
	state := url.Query().Get("state")
	errStr := url.Query().Get("error")
	errDesc := url.Query().Get("error_description")

	orgId := state

	// Redirect to dashboard
	if errStr != "" {
		log.Error("Error from stripe for org %v: %v", orgId, errStr, ctx)
		c.Redirect(302, fmt.Sprintf("%v/dash/integrations?error=%v", config.DashboardUrl, errStr+":"+errDesc))
		return
	}

	// Failed to get back authorization code from Stripe
	if err := org.GetById(orgId); err != nil {
		log.Error("Failed fetch organization: %v", err, ctx)
		c.Redirect(302, fmt.Sprintf("%v/dash/integrations?error=%v", config.DashboardUrl, err))
		return
	}

	token, testToken, err := connect.GetTokens(ctx, code)
	if err != nil {
		log.Error("params: %v %v", code, state, ctx)
		log.Error("Error from stripe connect for org %v: %v", orgId, err, ctx)
		c.Redirect(302, fmt.Sprintf("%v/dash/integrations?error=%v", config.DashboardUrl, err))
		return
	}

	if in := org.Integrations.FindByType(integration.StripeType); in == nil {
		in = &integration.Integration{
			Stripe: integration.Stripe{
				UserId:         token.UserId,
				AccessToken:    token.AccessToken,
				PublishableKey: token.PublishableKey,
				RefreshToken:   token.RefreshToken,
				Live:           *token,
				Test:           *testToken,
			},
			Type:    integration.StripeType,
			Enabled: true,
			Id:      rand.ShortId(),
		}
		if ins, err := org.Integrations.Append(in); err != nil {
			log.Error("Error adding stripe integration for %v: %v", orgId, err, ctx)
			c.Redirect(302, fmt.Sprintf("%v/dash/integrations?error=%v", config.DashboardUrl, err))
			return
		} else {
			org.Integrations = ins
		}

		// this needs to be nuked at some point
		org.Stripe = in.Stripe
	} else {
		in.Stripe.UserId = token.UserId
		in.Stripe.AccessToken = token.AccessToken
		in.Stripe.PublishableKey = token.PublishableKey
		in.Stripe.RefreshToken = token.RefreshToken
		in.Stripe.Live = *token
		in.Stripe.Test = *testToken
		in.Enabled = true
		if ins, err := org.Integrations.Update(in); err != nil {
			log.Error("Error updating stripe integration for %v: %v", orgId, err, ctx)
			c.Redirect(302, fmt.Sprintf("%v/dash/integrations?error=%v", config.DashboardUrl, err))
			return
		} else {
			org.Integrations = ins
		}

		// this needs to be nuked at some point
		org.Stripe = in.Stripe
	}

	if err := org.Update(); err != nil {
		log.Error("Error updating organization %v: %v", orgId, err, ctx)
		c.Redirect(302, fmt.Sprintf("%v/dash/integrations?error=%v", config.DashboardUrl, err))
		return
	}

	// Write Stripe credentials to KMS
	if v, ok := c.Get("kms"); ok {
		if kmsClient, ok := v.(*kms.CachedClient); ok {
			client := kmsClient.Client()
			path := "/tenants/" + org.Name + "/stripe"
			if org.Stripe.Live.AccessToken != "" {
				client.SetSecret(path, "STRIPE_LIVE_ACCESS_TOKEN", org.Stripe.Live.AccessToken)
			}
			if org.Stripe.Test.AccessToken != "" {
				client.SetSecret(path, "STRIPE_TEST_ACCESS_TOKEN", org.Stripe.Test.AccessToken)
			}
			if org.Stripe.PublishableKey != "" {
				client.SetSecret(path, "STRIPE_PUBLISHABLE_KEY", org.Stripe.PublishableKey)
			}
		}
	}

	c.Redirect(302, fmt.Sprintf("%v/dash/integrations?success=true&type=%v", config.DashboardUrl, integration.StripeType))
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
	ctx := middleware.GetContext(c)
	db := datastore.New(ctx)
	org := organization.New(db)

	// Failed to get back authorization code from Stripe
	if err := org.GetById(orgId); err != nil {
		log.Error("Failed fetch organization: %v", err, c)
		c.Redirect(302, org.Affiliate.ErrorUrl)
		return
	}

	// Fetch affiliate
	// Note: namespace handling removed - implement alternative if needed
	db = datastore.New(ctx)
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
