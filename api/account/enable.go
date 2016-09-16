package account

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/token"
	"crowdstart.com/models/user"
	"crowdstart.com/util/emails"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"
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

func enable(c *gin.Context) {
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

	if org.SignUpOptions.TwoStageEnabled {
		usr.Email = strings.ToLower(strings.TrimSpace(usr.Email))

		req := &twoStageEnableReq{User: usr}

		if err := json.Decode(c.Request.Body, req); err != nil {
			http.Fail(c, 400, "Failed decode request body", err)
			return
		}

		if req.Password != "" {
			if err := resetPassword(usr, req); err != nil {
				switch err {
				case PasswordMismatchError, PasswordMinLengthError:
					http.Fail(c, 400, err.Error(), err)
					return
				}
			}
		}
	}

	// Set user as enabled
	usr.Enabled = true
	if err := usr.Put(); err != nil {
		http.Fail(c, 500, "Failed to enable user", err)
		return
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
	ctx := middleware.GetAppEngine(c)
	emails.SendEmailConfirmedEmail(ctx, org, usr)

	loginTok := middleware.GetToken(c)
	loginTok.Set("user-id", usr.Id())
	loginTok.Set("exp", time.Now().Add(time.Hour*24*7))

	http.Render(c, 200, gin.H{"status": "ok", "token": loginTok.String()})
}
