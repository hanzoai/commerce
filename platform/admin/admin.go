package admin

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models2/order"
	"crowdstart.io/models2/product"
	"crowdstart.io/util/log"
	"crowdstart.io/util/permission"
	"crowdstart.io/util/template"
)

// Index
func Index(c *gin.Context) {
	url := config.UrlFor("platform", "/dashboard")
	log.Debug("Redirecting to %s", url)
	c.Redirect(301, url)
}

// Admin Dashboard
func Dashboard(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	org := middleware.GetOrganization(c)

	template.Render(c, "admin/dashboard.html", "org", org, "user", user)
}

func Product(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	org := middleware.GetOrganization(c)

	db := datastore.New(org.Namespace(c))

	p := product.New(db)
	id := c.Params.ByName("id")
	p.Get(id)

	template.Render(c, "admin/product.html", "org", org, "user", user, "product", p)
}

func Products(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	org := middleware.GetOrganization(c)

	template.Render(c, "admin/list-products.html", "org", org, "user", user)
}

func Order(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	org := middleware.GetOrganization(c)

	db := datastore.New(org.Namespace(c))

	o := order.New(db)
	id := c.Params.ByName("id")
	o.Get(id)

	template.Render(c, "admin/order.html", "org", org, "user", user, "order", o)
}

func Orders(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	org := middleware.GetOrganization(c)

	template.Render(c, "admin/list-orders.html", "org", org, "user", user)
}

func Organization(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	org := middleware.GetOrganization(c)

	template.Render(c, "admin/organization.html", "org", org, "user", user)
}

func Keys(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	org := middleware.GetOrganization(c)

	template.Render(c, "admin/keys.html", "org", org, "user", user)
}

func NewKeys(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	org := middleware.GetOrganization(c)

	org.ClearTokens()
	org.AddToken("live-secret-key", permission.Admin)
	org.AddToken("live-published-key", permission.Published)
	org.AddToken("test-secret-key", permission.Admin)
	org.AddToken("test-published-key", permission.Published)

	if err := org.Put(); err != nil {
		panic(err)
	}

	template.Render(c, "admin/keys.html", "org", org, "user", user)
}
