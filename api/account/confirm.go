package account

import (
	"errors"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/token"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
)

// Copy to Hanzo
var (
	ErrPasswordMinLength = errors.New("password needs to be atleast 6 characters")
	ErrPasswordMismatch  = errors.New("passwords need to match")
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
		return ErrPasswordMinLength
	}

	if req.GetPassword() != req.GetPasswordConfirm() {
		return ErrPasswordMismatch
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

func confirm(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	usr := user.New(db)
	tok := token.New(db)

	// Get Token
	id := c.Param("tokenid")
	if err := tok.GetById(id); err != nil {
		panic(err)
	}

	// Get user associated with token
	if err := usr.GetById(tok.UserId); err != nil {
		panic(err)
	}

	if tok.Expired() || tok.Used {
		return http.Fail(c, 403, "Token expired", errors.New("token expired"))
	}

	// Get new password
	req := &confirmPasswordReq{}
	if err := json.DecodeBytes(c.Body(), req); err != nil {
		return http.Fail(c, 400, "Failed decode request body", err)
	}

	if err := resetPassword(usr, req); err != nil {
		switch err {
		case ErrPasswordMismatch, ErrPasswordMinLength:
			return http.Fail(c, 400, err.Error(), err)
		default:
			return http.Fail(c, 500, err.Error(), err)
		}
	}

	// Save token
	tok.Used = true
	if err := tok.Put(); err != nil {
		log.Warn("Unable to update token", err, c)
	}

	// Return a new token with user id set
	loginTok := middleware.GetToken(c)
	loginTok.UserId = usr.Id()
	loginTok.ExpirationTime = time.Now().Add(time.Hour * 24 * 7).Unix()

	return http.Render(c, 200, map[string]any{"status": "ok", "token": loginTok.Encode(org.SecretKey)})
}
