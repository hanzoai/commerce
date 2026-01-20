package middleware

import (
	"context"
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/types/accesstoken"
	"github.com/hanzoai/commerce/util/session"
)

func AcquireOrganization(moduleName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := GetCurrentUser(c)

		// How did you get this far without an organization, bruh?
		if len(u.Organizations) < 1 {
			panic("THE WORLD MAKES NO SENSE.")
		}

		log.Debug("Found user")

		// Try and re-use last organization
		orgId := session.GetString(c, "active-organization")

		// Or default to their first organization
		if orgId == "" {
			orgId = u.Organizations[0]
		}

		// Fetch organization
		db := datastore.New(c)
		org := organization.New(db)
		err := org.GetById(orgId)
		if err != nil {
			log.Warn("Unable to acquire organization.", c)
			session.Clear(c)
			c.Redirect(302, config.UrlFor(moduleName, "/login"))
			c.AbortWithStatus(302)
		} else {
			log.Debug("Organization acquired")
			c.Set("user", u)
			c.Set("organization", org)
			c.Set("active-organization", org.Id())

			session.Set(c, "active-organization", org.Id())

			// Set for our readme integration
			c.Writer.Header().Set("x-readme-id", org.Id())
			c.Writer.Header().Set("x-readme-label", org.Name)
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

func GetToken(c *gin.Context) *accesstoken.AccessToken {
	return c.MustGet("token").(*accesstoken.AccessToken)
}

func GetNamespace(c *gin.Context) context.Context {
	ctx := GetAppEngine(c)
	return GetOrganization(c).Namespaced(ctx)
}
