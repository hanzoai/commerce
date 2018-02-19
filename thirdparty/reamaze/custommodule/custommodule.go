package custommodule

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"strings"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/order"
	"hanzo.io/models/user"
	"hanzo.io/util/json/http"
	"hanzo.io/log"
	"hanzo.io/util/template"
)

func Serve(c *context.Context) {
	query := c.Request.URL.Query()
	email := query.Get("email")

	if email == "" {
		log.Warn("No email provided", c)
		http.Fail(c, 400, "No email provided", errors.New("No email provided"))
		return
	}

	email = strings.ToLower(email)

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

	for _, ord := range ords {
		ord.Init(db)
		pays, _ := ord.GetPayments()
		ord.Payments = pays
	}

	template.Render(c, "reamaze/index.html", "usr", usr, "ords", ords)
}
