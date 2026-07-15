package account

import (
	"errors"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/email"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/token"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
)

type twoStageEnableReq struct {
	*user.User

	Password        string `json:"password"`
	PasswordConfirm string `json:"passwordConfirm"`
}

func (r twoStageEnableReq) GetPassword() string {
	return r.Password
}

func (r twoStageEnableReq) GetPasswordConfirm() string {
	return r.PasswordConfirm
}

func enable(c *zip.Ctx) error {
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

	if org.SignUpOptions.TwoStageEnabled {
		usr.Email = strings.ToLower(strings.TrimSpace(usr.Email))

		req := &twoStageEnableReq{User: usr}

		if err := json.DecodeBytes(c.Body(), req); err != nil {
			return http.Fail(c, 400, "Failed decode request body", err)
		}

		if req.Password != "" {
			if err := resetPassword(usr, req); err != nil {
				switch err {
				case ErrPasswordMismatch, ErrPasswordMinLength:
					return http.Fail(c, 400, err.Error(), err)
				}
			}
		}
	}

	// Set user as enabled
	usr.Enabled = true
	if err := usr.Put(); err != nil {
		return http.Fail(c, 500, "Failed to enable user", err)
	}

	// Token reuseable if no password is set
	if len(usr.PasswordHash) > 0 {
		// Save token
		tok.Used = true
		if err := tok.Put(); err != nil {
			log.Warn("Unable to update token", err, c)
		}
	}

	// Send account confirmed email
	ctx := c.Context()
	email.SendUserActivated(ctx, org, usr)

	loginTok := middleware.GetToken(c)
	loginTok.UserId = usr.Id()
	loginTok.ExpirationTime = time.Now().Add(time.Hour * 24 * 7).Unix()

	return http.Render(c, 200, map[string]any{"status": "ok", "token": loginTok.Encode(org.SecretKey)})
}
