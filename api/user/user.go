package user

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/transaction"
	"crowdstart.com/util/json/http"
)

func getTransactions(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespace(c))
	id := c.Params.ByName("userid")

	var trans []transaction.Transaction
	if _, err := transaction.Query(db).Filter("Test=", false).Filter("UserId=", id).GetAll(&trans); err != nil {
		http.Fail(c, 400, "Could not query transaction", err)
		return
	}

	http.Render(c, 200, trans)
}
