package api

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/user"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
)

func Fail(c *gin.Context, code int, message string, err error) {
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Write(json.EncodeBytes(gin.H{"code": code, "message": message}))

	if err != nil {
		log.Error(message+": %v", err, c)
	}

	c.Abort(code)
}

func authorize(c *gin.Context, id, email, password string) {
	db := datastore.New(c)
	u := user.New(db)

	// Try to get user by email
	if err := u.GetByEmail(email); err != nil {
		Fail(c, 401, "Invalid email address.", nil)
		return
	}

	// Check password
	if err := auth.CompareHashAndPassword(u.PasswordHash, password); err != nil {
		Fail(c, 401, "Invalid password.", nil)
		return
	}

	// Get organization
	org := organization.New(db)
	if err := org.Get(id); err != nil {
		Fail(c, 500, "Unable to retrieve organization", err)
		return
	}

	// Generate a new access token
	accessToken, err := org.GenerateAccessToken(u)
	if err != nil {
		Fail(c, 500, "Unable to generate access token", err)
		return
	}

	c.JSON(200, gin.H{"status": "ok", "token": accessToken})
}
