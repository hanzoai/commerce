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

var (
	PasswordMinLengthError = errors.New("Password needs to be atleast 6 characters")
	PasswordMismatchError  = errors.New("Passwords need to match")
)

type confirmPasswordReq struct {
	*user.User

	Password        string `json:"password"`
	PasswordConfirm string `json:"passwordConfirm"`
}

func resetPassword(usr *user.User, req *confirmPasswordReq) error {
	// Validate password
	if len(req.Password) < 6 {
		return PasswordMinLengthError
	}

	if req.Password != req.PasswordConfirm {
		return PasswordMismatchError
	}

	// Update password
	if err := usr.SetPassword(req.Password); err != nil {
		return err
	}

	// Enable user in case this user has never confirmed account
	usr.Enabled = true

	if err := usr.Put(); err != nil {
		return err
	}

	return nil
}

func confirm(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

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
	req := &confirmPasswordReq{}
	if err := json.Decode(c.Request.Body, req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	if err := resetPassword(usr, req); err != nil {
		switch err {
		case PasswordMismatchError, PasswordMinLengthError:
			http.Fail(c, 400, err.Error(), err)
		default:
			http.Fail(c, 500, err.Error(), err)
		}
		return
	}

	// Save token
	tok.Used = true
	if err := tok.Put(); err != nil {
		log.Warn("Unable to update token", err, c)
	}

	http.Render(c, 200, gin.H{"status": "ok"})
}
