package admin

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth2"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/models2/organization"
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
	if u, err := auth.GetCurrentUser(c); err != nil {
		c.Fail(500, err)
	} else {
		template.Render(c, "admin/dashboard.html", "user", u)
	}
}

func Product(c *gin.Context) {
	if u, err := auth.GetCurrentUser(c); err != nil {
		c.Fail(500, err)
	} else {
		db := datastore.New(c)
		org := organization.New(db)
		if _, err := org.Query().Filter("OwnerId=", u.Id()).First(); err != nil {
			c.Fail(500, err)
			return
		}

		namespaced := org.Namespace(c)
		db = datastore.New(namespaced)
		product := product.New(db)
		id := c.Params.ByName("id")
		log.Warn("%v", id)
		product.Get(id)

		template.Render(c, "admin/product.html", "org", org, "product", product)
	}
}

func Products(c *gin.Context) {
	if u, err := auth.GetCurrentUser(c); err != nil {
		c.Fail(500, err)
	} else {
		db := datastore.New(c)
		org := organization.New(db)
		if _, err := org.Query().Filter("OwnerId=", u.Id()).First(); err != nil {
			c.Fail(500, err)
			return
		}

		template.Render(c, "admin/list-products.html", "org", org)
	}
}

func Orders(c *gin.Context) {
	if u, err := auth.GetCurrentUser(c); err != nil {
		c.Fail(500, err)
	} else {
		db := datastore.New(c)
		org := organization.New(db)
		if _, err := org.Query().Filter("OwnerId=", u.Id()).First(); err != nil {
			c.Fail(500, err)
			return
		}

		template.Render(c, "admin/list-orders.html", "org", org)
	}
}

func Organization(c *gin.Context) {
	if u, err := auth.GetCurrentUser(c); err != nil {
		c.Fail(500, err)
	} else {
		db := datastore.New(c)
		org := organization.New(db)
		if _, err := org.Query().Filter("OwnerId=", u.Id()).First(); err != nil {
			c.Fail(500, err)
			return
		}

		template.Render(c, "admin/organization.html", "org", org)
	}
}

func Keys(c *gin.Context) {
	u, err := auth.GetCurrentUser(c)
	if err != nil {
		panic(err)
	}
	db := datastore.New(c)
	org := organization.New(db)
	if _, err := org.Query().Filter("OwnerId=", u.Id()).First(); err != nil {
		panic(err)
	}

	template.Render(c, "admin/keys.html", "org", org)
}

func NewKeys(c *gin.Context) {
	u, err := auth.GetCurrentUser(c)
	if err != nil {
		panic(err)
	}

	db := datastore.New(c)
	org := organization.New(db)
	if _, err := org.Query().Filter("OwnerId=", u.Id()).First(); err != nil {
		panic(err)
	}

	org.ClearTokens()
	org.AddToken("live-secret-key", permission.Admin)
	org.AddToken("live-published-key", permission.Published)
	org.AddToken("test-secret-key", permission.Admin)
	org.AddToken("test-published-key", permission.Published)

	if err := org.Put(); err != nil {
		panic(err)
	}

	template.Render(c, "admin/keys.html", "org", org)
}
