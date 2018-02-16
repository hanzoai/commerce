package affiliate

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/order"
	"hanzo.io/models/referral"
	"hanzo.io/models/referrer"
	"hanzo.io/models/transaction"
	"hanzo.io/models/user"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"

	"hanzo.io/models/affiliate"
	"hanzo.io/util/rest"
)

const (
	stripeConnectUrl = "https://connect.stripe.com/oauth/authorize?response_type=code&client_id=%s&scope=read_write&stripe_landing=login&redirect_uri=%s&state=%s"
)

//<a href="api.hanzo.io/affiliate/:id/connect"></a>

func connect(c *context.Context) {
	id := c.Params.ByName("affiliateid")
	org := middleware.GetOrganization(c)
	state := org.Id() + ":" + id
	url := fmt.Sprintf(stripeConnectUrl, config.Stripe.ClientId, config.Stripe.RedirectURL, state)
	c.Redirect(302, url)
}

func getReferrals(c *context.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	id := c.Params.ByName("affiliateid")

	referrals := make([]referral.Referral, 0)
	if _, err := referral.Query(db).Filter("Referrer.AffiliateId=", id).GetAll(&referrals); err != nil {
		http.Fail(c, 400, "Could not query referral", err)
		return
	}

	http.Render(c, 200, referrals)
}

func getReferrers(c *context.Context) {
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

func getOrders(c *context.Context) {
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

func getTransactions(c *context.Context) {
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

func create(r *rest.Rest) func(*context.Context) {
	return func(c *context.Context) {
		if !r.CheckPermissions(c, "create") {
			return
		}

		db := datastore.New(c)
		aff := affiliate.New(db)

		// Decode request
		if err := json.Decode(c.Request.Body, aff); err != nil {
			r.Fail(c, 400, "Failed decode request body", err)
			return
		}

		// Affiliates can only be created for pre-existing users
		if aff.UserId == "" {
			r.Fail(c, 500, "UserId required", errors.New("UserId required"))
			return
		}

		// Find user which will be turned into affiliate
		usr := user.New(db)
		if err := usr.GetById(aff.UserId); err != nil {
			r.Fail(c, 500, "User does not exist: "+aff.UserId, err)
			return
		}

		// Don't create multiple affiliates per user
		if usr.AffiliateId != "" {
			r.Fail(c, 500, "User already is affiliate: "+usr.AffiliateId, errors.New("User already is affiliate: "+usr.AffiliateId))
			return
		}

		// Create affiliate
		if err := aff.Create(); err != nil {
			r.Fail(c, 500, "Failed to create "+r.Kind, err)
			return
		}

		// Create special referrer for affiliate
		ref := referrer.New(db)
		ref.AffiliateId = aff.Id()
		ref.UserId = usr.Id()
		if err := ref.Create(); err != nil {
			r.Fail(c, 500, "Failed to create "+r.Kind, err)
			return
		}

		// Update user with affiliate information
		usr.AffiliateId = aff.Id()
		if err := usr.Update(); err != nil {
			r.Fail(c, 500, "Failed to update user: "+usr.Id(), err)
		}

		c.Writer.Header().Add("Location", c.Request.URL.Path+"/"+aff.Id())
		r.Render(c, 201, aff)
	}
}
