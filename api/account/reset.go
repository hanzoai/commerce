package account

import (
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

type resetReq struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Id       string `json:"id"`
}

func reset(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	usr := user.New(db)

	// Get new password
	req := &resetReq{}
	if err := json.DecodeBytes(c.Body(), req); err != nil {
		return http.Fail(c, 400, "Failed decode request body", err)
	}

	emailAddress := req.Email

	if err := usr.GetByEmail(emailAddress); err != nil {
		// If user doesn't exist, we pretend like it's ok
		log.Warn("Email doesn't exist, unable to reset password: %v", emailAddress, c)
		return http.Render(c, 200, map[string]any{"status": "ok"})
	}

	// Create token
	tok := token.New(usr.Datastore())
	tok.Email = usr.Email
	tok.UserId = usr.Id()
	tok.Expires = time.Now().Add(time.Hour * 72)

	if err := tok.Put(); err != nil {
		return http.Fail(c, 500, "Unable to create reset token", err)
	}

	// Send email
	ctx := c.Context()
	email.SendResetPassword(ctx, org, usr, tok)

	return http.Render(c, 200, map[string]any{"status": "ok"})
}
