package admin

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models2/order"
	"crowdstart.io/models2/product"
	"crowdstart.io/models2/user"
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
	template.Render(c, "admin/dashboard.html")
}

func Product(c *gin.Context) {
	db := datastore.New(middleware.GetNamespace(c))

	p := product.New(db)
	id := c.Params.ByName("id")
	p.Get(id)

	template.Render(c, "admin/product.html", "product", p)
}

func Products(c *gin.Context) {
	template.Render(c, "admin/list-products.html")
}

func Stores(c *gin.Context) {
	template.Render(c, "admin/list-stores.html")
}

func Order(c *gin.Context) {
	db := datastore.New(middleware.GetNamespace(c))

	o := order.New(db)
	id := c.Params.ByName("id")
	o.Get(id)

	u := user.New(db)
	u.Get(o.UserId)

	template.Render(c, "admin/order.html", "order", o, "user", u)
}

func Orders(c *gin.Context) {
	template.Render(c, "admin/list-orders.html")
}

func Organization(c *gin.Context) {
	template.Render(c, "admin/organization.html")
}

func Keys(c *gin.Context) {
	template.Render(c, "admin/keys.html")
}

func NewKeys(c *gin.Context) {
	org := middleware.GetOrganization(c)

	org.ClearTokens()
	org.AddToken("live-secret-key", permission.Admin)
	org.AddToken("live-published-key", permission.Published)
	org.AddToken("test-secret-key", permission.Admin)
	org.AddToken("test-published-key", permission.Published)

	if err := org.Put(); err != nil {
		panic(err)
	}

	template.Render(c, "admin/keys.html")
}
