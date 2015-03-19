package admin

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth2"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/models2/organization"
	"crowdstart.io/util/log"
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
	if u, err := auth.GetCurrentUser(c); err != nil {
		c.Fail(500, err)
	} else {
		db := datastore.New(c)
		org := organization.New(db)
		if _, err := org.Query().Filter("OwnerId=", u.Id()).First(); err != nil {
			c.Fail(500, err)
			return
		}

		template.Render(c, "admin/keys.html", "org", org)
	}
}
