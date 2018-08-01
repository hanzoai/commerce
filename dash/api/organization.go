package api

import (
	"errors"
	"regexp"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/log"
	"hanzo.io/util/session"
)

var verusEmailRe = regexp.MustCompile("@verus.io$|@hanzo.io$")

// Check if user is allowed to switch to organization
func validOrganization(u *user.User, orgId string) bool {
	// Verus/Crowdstart user
	if verusEmailRe.MatchString(u.Email) {
		return true
	}

	for _, id := range u.Organizations {
		if orgId == id {
			return true
		}
	}

	// YOU SHALL NOT PASS.
	return false
}

func Organization(c *gin.Context) {
	o := middleware.GetOrganization(c)

	org := new(organization.Organization)
	org.Name = o.Name
	org.FullName = o.FullName
	org.Websites = o.Websites
	org.EmailWhitelist = o.EmailWhitelist

	http.Render(c, 200, org)
}

func UpdateOrganization(c *gin.Context) {
	o := new(organization.Organization)
	if err := json.Decode(c.Request.Body, o); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	org := middleware.GetOrganization(c)

	org.FullName = o.FullName
	org.Websites = o.Websites
	org.EmailWhitelist = o.EmailWhitelist

	org.Put()

	c.Writer.WriteHeader(204)
}

func SetActiveOrganization(c *gin.Context) {
	orgId := c.Params.ByName("organizationid")

	db := datastore.New(c)
	u := middleware.GetCurrentUser(c)

	if !validOrganization(u, orgId) {
		msg := "You do not have permission to switch to that organization"
		err := errors.New(msg)
		http.Fail(c, 400, msg, err)
		return
	}

	org := organization.New(db)
	err := org.GetById(orgId)
	if err != nil {
		log.Warn("Tried to switch to invalid organization: '%v'", orgId)
		session.Clear(c)
		http.Fail(c, 400, "Failed decode request body", err)
		return
	} else {
		log.Debug("Set active organization to '%v'", orgId, c)
		session.Set(c, "active-organization", org.Id())
	}

	http.Render(c, 200, org)
}
