package user

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/auth/password"
	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/order"
	"hanzo.io/models/referral"
	"hanzo.io/models/referrer"
	"hanzo.io/models/subscription"
	"hanzo.io/models/transaction"
	"hanzo.io/models/user"
	"hanzo.io/util/json/http"
	"hanzo.io/util/rand"
)

type Password struct {
	Password string `json:"password"`
}

func resetPassword(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	id := c.Params.ByName("userid")

	u := user.New(db)
	if err := u.GetById(id); err != nil {
		http.Fail(c, 400, "Could not query user", err)
		return
	}

	newPassword := rand.ShortPassword()
	if hash, err := password.Hash(newPassword); err != nil {
		http.Fail(c, 400, "Password generation failed", err)
		return
	} else {
		u.PasswordHash = hash
	}

	u.MustPut()
	http.Render(c, 200, Password{Password: newPassword})
}

func getReferrals(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	id := c.Params.ByName("userid")

	referrals, err := referral.Query(db).Filter("ReferrerUserId=", id).GetAll()
	if err != nil {
		http.Fail(c, 400, "Could not query referral", err)
		return
	}

	http.Render(c, 200, referrals)
}

func getReferrers(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	id := c.Params.ByName("userid")

	referrers, err := referrer.Query(db).Filter("UserId=", id).GetAll()
	if err != nil {
		http.Fail(c, 400, "Could not query referrer", err)
		return
	}

	http.Render(c, 200, referrers)
}

func getSubscriptions(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	id := c.Params.ByName("userid")

	subscriptions, err := subscription.Query(db).Filter("UserId=", id).GetAll()
	if err != nil {
		http.Fail(c, 400, "Could not query subscription", err)
		return
	}

	http.Render(c, 200, subscriptions)
}

func getOrders(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	id := c.Params.ByName("userid")

	orders, err := order.Query(db).Filter("UserId=", id).GetAll()
	if err != nil {
		http.Fail(c, 400, "Could not query order", err)
		return
	}

	http.Render(c, 200, orders)
}

func getTransactions(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	id := c.Params.ByName("userid")

	trans, err := transaction.Query(db).Filter("Test=", false).Filter("UserId=", id).GetAll()
	if err != nil {
		http.Fail(c, 400, "Could not query transaction", err)
		return
	}

	http.Render(c, 200, trans)
}
