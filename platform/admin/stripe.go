package admin

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/thirdparty/stripe/connect"
	"crowdstart.com/thirdparty/stripe/tasks"
	"crowdstart.com/util/log"
	"crowdstart.com/util/template"
)

/*
Warning
Due to the fact that `CampaignId`s are currently missing in all the orders,
this function assumes that every order is associated with the only campaign (SKULLY).

TODO: Run a migration to set `CampaignId` in all orders.
*/
func StripeSync(c *gin.Context) {
	ctx := middleware.GetAppEngine(c)
	tasks.SyncCharges.Call(ctx, "organization", middleware.GetOrganization(c).Id())
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

	// Get live and test tokens
	token, testToken, err := connect.GetTokens(ctx, code)
	if err != nil {
		log.Error("There was an error with Stripe Connect: %v", err, c)
		template.Render(c, "admin/stripe/connect.html", "stripeError", err)
		return
	}

	// Get user's organization
	org := middleware.GetOrganization(c)

	// Update stripe data
	org.Stripe.UserId = token.UserId
	org.Stripe.AccessToken = token.AccessToken
	org.Stripe.PublishableKey = token.PublishableKey
	org.Stripe.RefreshToken = token.RefreshToken

	// Save live/test tokens
	org.Stripe.Live = *token
	org.Stripe.Test = *testToken

	// Save to datastore
	if err := org.Put(); err != nil {
		c.Fail(500, err)
		return
	}

	// Success
	template.Render(c, "admin/stripe/connect.html")
}
