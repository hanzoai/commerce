package middleware

import (
	"context"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/types/accesstoken"
	"github.com/hanzoai/commerce/util/session"
)

func AcquireOrganization(moduleName string) zip.Handler {
	return func(c *zip.Ctx) error {
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
		db := datastore.New(c.Context())
		org := organization.New(db)
		if err := org.GetById(orgId); err != nil {
			log.Warn("Unable to acquire organization.", c)
			session.Clear(c)
			return c.Redirect(302, config.UrlFor(moduleName, "/login"))
		}

		log.Debug("Organization acquired")
		c.Locals("user", u)
		c.Locals("organization", org)
		c.Locals("active-organization", org.Id())

		session.Set(c, "active-organization", org.Id())

		// Set for our readme integration
		c.SetHeader("x-readme-id", org.Id())
		c.SetHeader("x-readme-label", org.Name)

		return c.Next()
	}
}

// Namespace applies the organization's namespace to the request context.
func Namespace() zip.Handler {
	return func(c *zip.Ctx) error {
		ctx := c.Context()
		org := GetOrganization(c)
		ctx = org.Namespaced(ctx)
		c.SetContext(ctx)
		return c.Next()
	}
}

func GetOrganization(c *zip.Ctx) *organization.Organization {
	return c.Locals("organization").(*organization.Organization)
}

// GetOrganizationOK returns the request organization without panicking when
// it is absent. Use this on handlers mounted outside the auth-token group
// (e.g. signature-verified webhook ingress) where no session has set an
// organization; GetOrganization would panic there.
func GetOrganizationOK(c *zip.Ctx) (*organization.Organization, bool) {
	v := c.Locals("organization")
	if v == nil {
		return nil, false
	}
	org, ok := v.(*organization.Organization)
	return org, ok
}

func GetToken(c *zip.Ctx) *accesstoken.AccessToken {
	return c.Locals("token").(*accesstoken.AccessToken)
}

func GetNamespace(c *zip.Ctx) context.Context {
	ctx := c.Context()
	return GetOrganization(c).Namespaced(ctx)
}
