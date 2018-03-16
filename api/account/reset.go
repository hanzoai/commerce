package account

import (
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/token"
	"hanzo.io/models/user"
	"hanzo.io/util/emails"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/log"
)

type resetReq struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Id       string `json:"id"`
}

func reset(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	usr := user.New(db)

	// Get new password
	req := &resetReq{}
	if err := json.Decode(c.Request.Body, req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	email := req.Email

	if err := usr.GetByEmail(email); err != nil {
		// If user doesn't exist, we pretend like it's ok
		log.Warn("Email doesn't exist, unable to reset password: %v", email, c)
		http.Render(c, 200, gin.H{"status": "ok"})
		return
	}

	// Create token
	tok := token.New(usr.Db)
	tok.Email = usr.Email
	tok.UserId = usr.Id()
	tok.Expires = time.Now().Add(time.Hour * 72)

	if err := tok.Put(); err != nil {
		http.Fail(c, 500, "Unable to create reset token", err)
		return
	}

	// Send email
	ctx := middleware.GetAppEngine(c)
	emails.SendPasswordResetEmail(ctx, org, usr, tok)

	http.Render(c, 200, gin.H{"status": "ok"})
}
