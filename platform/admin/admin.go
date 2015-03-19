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

// // Register
// func Register(c *gin.Context) {
// 	template.Render(c, "register.html")
// }

// // Post registration form
// func SubmitRegister(c *gin.Context) {
// 	c.Redirect(301, "dashboard")
// }

// Admin Dashboard
func Dashboard(c *gin.Context) {
	if u, err := auth.GetCurrentUser(c); err != nil {
		c.Fail(500, err)
	} else {
		template.Render(c, "dashboard.html",
			"user", u)
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

		template.Render(c, "organization.html",
			"org", org)
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

		template.Render(c, "keys.html",
			"org", org)
	}
}

// Theme Testing
func ThemeSample(c *gin.Context) {
	template.Render(c, "sample.html")
}
