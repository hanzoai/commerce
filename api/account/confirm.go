package account

import (
	"errors"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/token"
	"crowdstart.com/models/user"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"
)

func confirm(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespace(c))

	usr := user.New(db)
	tok := token.New(db)

	// Get Token
	id := c.Params.ByName("tokenid")
	if err := tok.GetById(id); err != nil {
		panic(err)
	}

	// Get user associated with token
	if err := usr.GetById(tok.UserId); err != nil {
		panic(err)
	}

	if tok.Expired() || tok.Used {
		http.Fail(c, 403, "Token expired", errors.New("Token expired"))
		return
	}

	// Get new password
	req := &resetReq{}
	if err := json.Decode(c.Request.Body, req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Validate password
	if len(req.Password) < 6 {
		http.Fail(c, 400, "Password needs to be atleast 6 characters", errors.New("Password needs to be atleast 6 characters"))
		return
	}

	if req.Password != req.PasswordConfirm {
		http.Fail(c, 400, "Passwords need to match", errors.New("Passwords need to match"))
		return
	}

	// Update password
	if err := usr.SetPassword(req.Password); err != nil {
		http.Fail(c, 500, "Failed to set password", err)
		return
	}

	// Enable user in case this user has never confirmed account
	usr.Enabled = true

	if err := usr.Put(); err != nil {
		http.Fail(c, 500, "Failed to update password", err)
		return
	}

	// Save token
	tok.Used = true
	if err := tok.Put(); err != nil {
		log.Warn("Unable to update token", err, c)
	}

	http.Render(c, 200, gin.H{"status": "ok"})
}
