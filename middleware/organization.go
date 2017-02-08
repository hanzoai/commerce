package middleware

import (
	"golang.org/x/net/context"

	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/util/log"
	"hanzo.io/util/session"
	"hanzo.io/util/token"
)

func AcquireOrganization(moduleName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := GetCurrentUser(c)

		// How did you get this far without an organization, bruh?
		if len(u.Organizations) < 1 {
			panic("THE WORLD MAKES NO SENSE.")
		}

		log.Debug("Found user.")

		// Try and re-use last organization
		orgId := session.GetString(c, "active-organization")

		// Or default to their first organization
		if orgId == "" {
			orgId = u.Organizations[0]
		}

		// Fetch organization
		db := datastore.New(c)
		org := organization.New(db)
		err := org.Get(orgId)
		if err != nil {
			log.Warn("Unable to acquire organization.")
			session.Clear(c)
			c.Redirect(302, config.UrlFor(moduleName, "/login"))
			c.AbortWithStatus(302)
		} else {
			log.Debug("Organization acquired")
			c.Set("user", u)
			c.Set("organization", org)
			c.Set("active-organization", org.Id())
			session.Set(c, "active-organization", org.Id())
		}
	}
}

// Automatically use namespace of organization set in context.
func Namespace() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := GetAppEngine(c)
		org := GetOrganization(c)
		ctx = org.Namespaced(ctx)
		c.Set("appengine", ctx)
	}
}

func GetOrganization(c *gin.Context) *organization.Organization {
	return c.MustGet("organization").(*organization.Organization)
}

func GetToken(c *gin.Context) *token.Token {
	return c.MustGet("token").(*token.Token)
}

func GetNamespaced(c *gin.Context) context.Context {
	ctx := GetAppEngine(c)
	return GetOrganization(c).Namespaced(ctx)
}
