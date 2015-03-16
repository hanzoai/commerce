package api

import (
	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/user"
	"crowdstart.io/util/json"
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

	// Generate a new access token
	accessToken, err := org.GenerateAccessToken(u)
	if err != nil {
		json.Fail(c, 500, "Unable to generate access token", err)
		return
	}

	// Save access token in cookie for ease of use during development
	if appengine.IsDevAppServer() {
		session.Set(c, "access-token", accessToken)
	}

	// Return access token
	json.Render(c, 200, gin.H{"status": "ok", "token": accessToken})
}
