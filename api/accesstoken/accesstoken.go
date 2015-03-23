package accesstoken

import (
	"errors"

	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/user"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/permission"
	"crowdstart.io/util/session"
)

func getAccessToken(c *gin.Context, id, email, password string) {
	db := datastore.New(c)
	u := user.New(db)

	// Try to get user by email
	if err := u.GetByEmail(email); err != nil {
		json.Fail(c, 401, "Invalid email address.", nil)
		return
	}

	// Check password
	if err := auth.CompareHashAndPassword(u.PasswordHash, password); err != nil {
		json.Fail(c, 401, "Invalid password.", nil)
		return
	}

	// Get organization
	org := organization.New(db)
	if err := org.Get(id); err != nil {
		json.Fail(c, 500, "Unable to retrieve organization", err)
		return
	}

	// Check if we have permission to create an access token
	if !(org.IsOwner(u) || org.IsAdmin(u)) {
		log.Warn("user (%v, %v) is not owner of (%v, %v)", u.Email, u.Id(), org.Name, org.Id())
		json.Fail(c, 500, "Must be owner or admin to create a new access token.", errors.New("Invalid user"))
		return
	}

	org.RemoveToken("live-secret-key")

	// Generate a new access token
	accessToken := org.AddToken("live-secret-key", permission.Admin)

	// Save organization
	org.Put()

	// Save access token in cookie for ease of use during development
	if appengine.IsDevAppServer() {
		session.Set(c, "access-token", accessToken)
	}

	// Return access token
	json.Render(c, 200, gin.H{"status": "ok", "token": accessToken})
}

func deleteAccessToken(c *gin.Context) {
	// Save access token in cookie for ease of use during development
	if appengine.IsDevAppServer() {
		session.Delete(c, "access-token")
	}

	// Get organization for current access token
	org := middleware.GetOrganization(c)

	// Retrieve token
	accessToken := session.MustGet(c, "access-token").(string)
	tok, err := org.GetToken(accessToken)
	if err != nil {
		json.Fail(c, 500, "Invalid token", err)
	}

	// Remove token
	org.RemoveToken(tok.Name)

	if err := org.Put(); err != nil {
		json.Fail(c, 500, "Unable to update organization", err)
	}

	// Return access token
	json.Render(c, 200, gin.H{"status": "ok"})
}
