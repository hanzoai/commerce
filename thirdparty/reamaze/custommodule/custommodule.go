package custommodule

import (
	"fmt"
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/order"
	"crowdstart.com/models/user"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"
	"crowdstart.com/util/template"
)

func Serve(c *gin.Context) {
	query := c.Request.URL.Query()
	email := query.Get("email")

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	usr := user.New(db)
	if err := usr.GetById(email); err != nil {
		log.Warn("User not found for email: %v", email, c)
		http.Fail(c, 400, fmt.Sprintf("User not found for email: %v", email), err)
		return
	}

	var ords []*order.Order
	if _, err := order.Query(db).Filter("UserId=", usr.Id()).GetAll(&ords); err != nil {
		log.Warn("Orders not found for email: %v", email, c)
		http.Fail(c, 400, fmt.Sprintf("Orders not found for email: %v", email), err)
		return
	}

	template.Render(c, "reamaze/index.html", "usr", usr, "ords", ords)
	c.String(200, "ok\n")
}
