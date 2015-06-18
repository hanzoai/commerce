package account

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/user"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
)

func get(c *gin.Context) {
	usr := middleware.GetUser(c)

	if err := usr.LoadReferrals(); err != nil {
		http.Fail(c, 500, "User referral data could get be queried", err)
		return
	}

	if err := usr.LoadOrders(); err != nil {
		http.Fail(c, 500, "User order data could get be queried", err)
		return
	}

	if err := usr.CalculateBalances(); err != nil {
		http.Fail(c, 500, "User balance data could get be queried", err)
		return
	}

	http.Render(c, 200, usr)
}

func update(c *gin.Context) {
	usr := middleware.GetUser(c)
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespace(c))

	id := usr.Id()
	newUsr := user.New(db)
	if err := json.Decode(c.Request.Body, newUsr); err != nil {
		newUsr.SetKey(id)
	}

	if err := newUsr.Put(); err != nil {
		http.Fail(c, 400, "Failed to update user", err)
	} else {
		http.Render(c, 200, usr)
	}
}

func patch(c *gin.Context) {
	usr := middleware.GetUser(c)
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespace(c))

	id := usr.Id()
	newUsr := user.New(db)
	if err := json.Decode(c.Request.Body, newUsr); err != nil {
		newUsr.SetKey(id)
	}

	if err := newUsr.Put(); err != nil {
		http.Fail(c, 400, "Failed to update user", err)
	} else {
		http.Render(c, 200, usr)
	}
}
