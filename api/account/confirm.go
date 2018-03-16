package account

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/token"
	"hanzo.io/models/user"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/log"
)

// Copy to Hanzo
var (
	PasswordMinLengthError = errors.New("Password needs to be atleast 6 characters")
	PasswordMismatchError  = errors.New("Passwords need to match")
)

type resetPasswordReq interface {
	GetPassword() string
	GetPasswordConfirm() string
}

type confirmPasswordReq struct {
	*user.User

	CurrentPassword string `json:"currentPassword"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"passwordConfirm"`
}

func (r confirmPasswordReq) GetPassword() string {
	return r.Password
}

func (r confirmPasswordReq) GetPasswordConfirm() string {
	return r.PasswordConfirm
}

func resetPassword(usr *user.User, req resetPasswordReq) error {
	// Validate password
	if len(req.GetPassword()) < 6 {
		return PasswordMinLengthError
	}

	if req.GetPassword() != req.GetPasswordConfirm() {
		return PasswordMismatchError
	}

	// Update password
	if err := usr.SetPassword(req.GetPassword()); err != nil {
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

	// Return a new token with user id set
	loginTok := middleware.GetToken(c)
	loginTok.Set("user-id", usr.Id())
	loginTok.Set("exp", time.Now().Add(time.Hour*24*7))

	http.Render(c, 200, gin.H{"status": "ok", "token": loginTok.String()})
}
