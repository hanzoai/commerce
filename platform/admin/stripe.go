package admin

import (
	"strings"

	"appengine"

	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/middleware"
	"crowdstart.com/thirdparty/stripe/connect"
	"crowdstart.com/thirdparty/stripe/tasks"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"
	"crowdstart.com/util/template"
)

func StripeSync(c *gin.Context) {
	ctx := middleware.GetAppEngine(c)
	org := middleware.GetOrganization(c)
	tasks.SyncCharges.Call(ctx, org.Id())

	c.Writer.WriteHeader(204)
}

type StripeData struct {
	State       string `json:"state"`
	ClientId    string `json:"clientId"`
	RedirectUrl string `json:"redirectUrl"`
}

// Admin Payment Connectors
func Stripe(c *gin.Context) {
	org := middleware.GetOrganization(c)

	sd := new(StripeData)
	sd.ClientId = config.Stripe.ClientId
	sd.RedirectUrl = config.Stripe.RedirectURL

	if org.Stripe.AccessToken == "" {
		sd.State = "new"
	} else {
		sd.State = "connected"
	}

	http.Render(c, 200, sd)
}

// Connect callback for platform
func StripeCallback(c *gin.Context) {
	req := c.Request
	code := req.URL.Query().Get("code")
	state := req.URL.Query().Get("state")
	errStr := req.URL.Query().Get("error")

	// Handle affiliate callbacks
	if state != "" {
		affiliateCallback(c)
		return
	}

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
		c.Redirect(302, config.UrlFor("platform", "dashboard#integrations"))
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
		c.AbortWithError(500, err)
		return
	}

	// Success
	c.Redirect(302, config.UrlFor("platform", "dashboard#integrations"))
}

// Connect callback for affiliates
func affiliateCallback(c *gin.Context) {
	req := c.Request
	code := req.URL.Query().Get("code")
	state := req.URL.Query().Get("state")
	errStr := req.URL.Query().Get("error")

	// Get organization and affiliate id
	parts := strings.Split(state, ":")
	orgName := parts[0]
	affId := pargs[1]

	// Fetch affiliate
	nsctx := appengine.Namespace(ctx, orgName)
	db := datastore.New(nsctx)
	aff := affiliate.New(db)
	aff.GetById(affid)

	// Failed to get back authorization code from Stripe
	if errStr != "" {
		log.Error("Failed to get authorization code from Stripe during Stripe Connect: %v", errStr, c)
		c.Redirect(302, org.Affilliate.ErrorUrl)
		return
	}

	// Get live and test tokens
	token, testToken, err := stripeconnect.GetTokens(ctx, code)
	if err != nil {
		log.Error("There was an error with Stripe Connect: %v", err, c)
		c.Redirect(302, org.Affilliate.ErrorUrl)
		return
	}

	// Update stripe data
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
		c.Redirect(302, org.Affilliate.ErrorUrl)
		return
	}

	// Success
	c.Redirect(302, org.Affilliate.SuccessUrl)
}
