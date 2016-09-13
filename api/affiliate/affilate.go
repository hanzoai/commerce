package affiliate

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/order"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/transaction"
	"crowdstart.com/models/user"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"

	"crowdstart.com/models/affiliate"
	"crowdstart.com/util/rest"
)

const (
	stripeConnectUrl = "https://connect.stripe.com/oauth/authorize?response_type=code&client_id=%s&scope=read_write&stripe_landing=login&redirect_uri=%s&state=%s"
)

//<a href="api.crowdstart.com/affiliate/:id/connect"></a>

func connect(c *gin.Context) {
	id := c.Params.ByName("affiliateid")
	org := middleware.GetOrganization(c)
	state := org.Id() + ":" + id
	url := fmt.Sprintf(stripeConnectUrl, config.Stripe.ClientId, config.Stripe.RedirectURL, state)
	c.Redirect(302, url)
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

func create(r *rest.Rest) func(*gin.Context) {
	return func(c *gin.Context) {
		if !r.CheckPermissions(c, "create") {
			return
		}

		db := datastore.New(c)
		aff := affiliate.New(db)

		if err := json.Decode(c.Request.Body, aff); err != nil {
			r.Fail(c, 400, "Failed decode request body", err)
			return
		}

		usr := user.New(db)

		// Create Mailchimp cart
		if aff.UserId == "" {
			r.Fail(c, 500, "UserId required", errors.New("UserId required"))
			return
		}

		if err := usr.GetById(aff.UserId); err != nil {
			r.Fail(c, 500, "User does not exist: "+aff.UserId, err)
			return
		}

		if usr.AffiliateId != "" {
			r.Fail(c, 500, "User already has affiliate: "+usr.AffiliateId, errors.New("User already has affiliate: "+usr.AffiliateId))
			return
		}

		if err := aff.Create(); err != nil {
			r.Fail(c, 500, "Failed to create "+r.Kind, err)
			return
		}

		usr.AffiliateId = aff.Id()

		if err := usr.Update(); err != nil {
			r.Fail(c, 500, "Failed to update user: "+usr.Id(), err)
		}

		c.Writer.Header().Add("Location", c.Request.URL.Path+"/"+aff.Id())
		r.Render(c, 201, aff)
	}
}

func enable(c *gin.Context) {
	id := c.Params.ByName("affiliateid")

	db := datastore.New(c)
	aff := affiliate.New(db)

	if err := aff.GetById(id); err != nil {
		http.Fail(c, 400, "Affiliate not found: "+id, err)
	}

	aff.Enabled = true

	aff.MustUpdate()

	http.Render(c, 201, aff)
}

func disable(c *gin.Context) {
	id := c.Params.ByName("affiliateid")

	db := datastore.New(c)
	aff := affiliate.New(db)

	if err := aff.GetById(id); err != nil {
		http.Fail(c, 400, "Affiliate not found: "+id, err)
	}

	aff.Enabled = true

	aff.MustUpdate()

	http.Render(c, 201, aff)
}
