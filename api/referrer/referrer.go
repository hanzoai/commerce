package referrer

import (
	"github.com/gin-gonic/gin"
	"hanzo.io/datastore"

	"hanzo.io/middleware"
	"hanzo.io/models/referral"
	"hanzo.io/models/transaction"
	"hanzo.io/util/json/http"
)

func getReferrals(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	id := c.Params.ByName("referrerid")

	if referrals, err := referral.Query(db).Filter("ReferrerId=", id).GetAll(); err != nil {
		http.Fail(c, 400, "Could not query referrals", err)
		return
	} else {
		http.Render(c, 200, referrals)
	}
}

func getTransactions(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	id := c.Params.ByName("referrerid")

	if transactions, err := transaction.Query(db).Filter("SourceId=", id).GetAll(); err != nil {
		http.Fail(c, 400, "Could not query transactions", err)
		return
	} else {
		http.Render(c, 200, transactions)
	}
}
