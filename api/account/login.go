package account

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"hanzo.io/auth/password"
	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/app"
	"hanzo.io/models/user"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/log"
)

type loginReq struct {
	Email           string `json:"email"`
	Id              string `json:"id"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"passwordConfirm"`
}

type loginRes struct {
	Token string `json:"token"`
}

func login(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	req := &loginReq{}

	// Decode response body to create new user
	if err := json.Decode(c.Request.Body, req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	var id string
	isEmail := false
	isUsername := false

	// Allow userame, email or id to be used to lookup user
	if req.Id != "" {
		id = req.Id
	} else if req.Email != "" {
		id = strings.ToLower(strings.TrimSpace(req.Email))
		isEmail = true
	} else if req.Username != "" {
		id = strings.ToLower(strings.TrimSpace(req.Username))
	} else {
		http.Fail(c, 400, "Could not find account", errors.New("Could not find account"))
		return
	}

	// Get user by email
	usr := user.New(db)

	// else {
	if err := usr.GetById(id); err != nil {
		log.Warn("Could not get by Id %v", id, c)
	}
	// }
	if isEmail {
		if err := usr.GetByEmail(id); err != nil {
			http.Fail(c, 401, "Email or password is incorrect", errors.New("Email or password is incorrect"))
			log.Debug("Unable to lookup user by email", c)
			return
		}
	} else if isUsername {
		if err := usr.GetByEmail(id); err != nil {
			http.Fail(c, 401, "Username or password is incorrect", errors.New("Email or password is incorrect"))
			log.Debug("Unable to lookup user by username", c)
			return
		}
	}

	// Check user's password
	if !password.HashAndCompare(usr.PasswordHash, req.Password) {
		http.Fail(c, 401, "Email or password is incorrect", errors.New("Email or password is incorrect"))
		log.Debug("Incorrect password", c)
		return
	}

	// If user is not enabled fail
	if !usr.Enabled {
		http.Fail(c, 401, "Account needs to be enabled", errors.New("Account needs to be enabled"))
		return
	}

	// Validate login claim set
	claims := middleware.GetClaims(c)

	ap := middleware.GetApp(c)
	tokName := app.PublishedKey
	if claims.Test {
		tokName = app.TestPublishedKey
	}

	tok, ok, err := ap.GetApiKeyByName(tokName)
	if !ok {
		log.Error(err, c)
		http.Fail(c, 401, "Api key has been revoked", errors.New("Api key has been revoked"))
		return
	}

	// Issue customer token
	aTokString, err := tok.IssueAccessToken(usr.Id(), ap.SecretKey)
	if err != nil {
		http.Fail(c, 500, "Could not issue login credentials", errors.New("Could not issue login credentials"))
		return
	}

	http.Render(c, 200, loginRes{aTokString})
}
