package affiliate

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/order"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/transaction"
	"crowdstart.com/util/json/http"

	"crowdstart.com/models/affiliate"
	stripeconnect "crowdstart.com/thirdparty/stripe/connect"
	"crowdstart.com/util/log"
)

const (
	stripeConnectUrl = "https://connect.stripe.com/oauth/authorize?response_type=code&client_id=%s&scope=read_write&state=%s&stripe_landing=login&redirect_uri=%s"
)

func connect(c *gin.Context) {
	id := c.Params.ByName("affiliateid")
	url := fmt.Sprintf(stripeConnectUrl, config.Stripe.ClientId, config.Stripe.RedirectURL, id)
	c.Redirect(302, url)
}

// Connect connect callback
func stripeCallback(c *gin.Context) {
	req := c.Request
	code := req.URL.Query().Get("code")
	affid := req.URL.Query().Get("state")
	errStr := req.URL.Query().Get("error")

	ctx := middleware.GetAppEngine(c)
	org := middleware.GetOrganization(c)
	db := datastore.New(c)
	aff := affiliate.New(db)
	aff.GetById(affid)

	// Failed to get back authorization code from Stripe
	if errStr != "" {
		log.Error("Failed to get authorization code from Stripe during Stripe Connect: %v", errStr, c)
		c.Redirect(302, org.AffilliateSettings.ErrorUrl)
		return
	}

	// Get live and test tokens
	token, testToken, err := stripeconnect.GetTokens(ctx, code)
	if err != nil {
		log.Error("There was an error with Stripe Connect: %v", err, c)
		c.Redirect(302, org.AffilliateSettings.ErrorUrl)
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
		c.Redirect(302, org.AffilliateSettings.ErrorUrl)
		return
	}

	// Success
	c.Redirect(302, org.AffilliateSettings.ConfirmUrl)
}

func getReferrals(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	id := c.Params.ByName("userid")

	referrals := make([]referral.Referral, 0)
	if _, err := referral.Query(db).Filter("ReferrerUserId=", id).GetAll(&referrals); err != nil {
		http.Fail(c, 400, "Could not query referral", err)
		return
	}

	http.Render(c, 200, referrals)
}

func getReferrers(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	id := c.Params.ByName("affiliateid")

	referrers := make([]referrer.Referrer, 0)
	if _, err := referrer.Query(db).Filter("AffiliateId=", id).GetAll(&referrers); err != nil {
		http.Fail(c, 400, "Could not query referrer", err)
		return
	}

	http.Render(c, 200, referrers)
}

func getOrders(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	id := c.Params.ByName("affiliateid")

	orders := make([]order.Order, 0)
	if _, err := order.Query(db).Filter("AffiliateId=", id).GetAll(&orders); err != nil {
		http.Fail(c, 400, "Could not query order", err)
		return
	}

	http.Render(c, 200, orders)
}

func getTransactions(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	id := c.Params.ByName("affiliateid")

	trans := make([]transaction.Transaction, 0)
	if _, err := transaction.Query(db).Filter("Test=", false).Filter("AffiliateId=", id).GetAll(&trans); err != nil {
		http.Fail(c, 400, "Could not query transaction", err)
		return
	}

	http.Render(c, 200, trans)
}
