package account

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/auth/password"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/user"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
)

type Token struct {
	Token string `json:"token"`
}

func get(c *gin.Context) {
	usr := middleware.GetUser(c)

	if err := usr.LoadReferrals(); err != nil {
		http.Fail(c, 500, "User referral data could get be queried", err)
		return
	}

	if err := usr.LoadOrders(); err != nil {
		http.Fail(c, 500, "User referral data could get be queried", err)
		return
	}

	http.Render(c, 200, usr)
}

func update(c *gin.Context) {
	usr := middleware.GetUser(c)
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespace(c))

	id := usr.Id()
	newUsr := user.New(db)
	if err := json.Decode(c.Request.Body, newUsr); err != nil {
		newUsr.SetKey(id)
	}

	if err := newUsr.Put(); err != nil {
		http.Fail(c, 400, "Failed to update user", err)
	} else {
		http.Render(c, 200, usr)
	}
}

func patch(c *gin.Context) {
	usr := middleware.GetUser(c)
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespace(c))

	id := usr.Id()
	newUsr := user.New(db)
	if err := json.Decode(c.Request.Body, newUsr); err != nil {
		newUsr.SetKey(id)
	}

	if err := newUsr.Put(); err != nil {
		http.Fail(c, 400, "Failed to update user", err)
	} else {
		http.Render(c, 200, usr)
	}
}

type userIn struct {
	*user.User

	Password        string `json:"password,omitempty"`
	PasswordConfirm string `json:"passwordConfirm,omitempty"`
}

func login(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespace(c))
	usr := user.New(db)

	usrIn := &userIn{User: usr}

	// Decode response body to create new user
	if err := json.Decode(c.Request.Body, usrIn); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	if err := usr.GetByEmail(usr.Email); err != nil {
		http.Fail(c, 401, "Email or password is incorrect", errors.New("Email or password is incorrect"))
		return
	}

	if !password.HashAndCompare(usr.PasswordHash, usrIn.Password) {
		http.Fail(c, 401, "Email or password is incorrect", errors.New("Email or password is incorrect"))
		return
	}

	if err := usr.GetByEmail(usr.Email); err != nil {
		http.Fail(c, 401, "Email or password is incorrect", errors.New("Email or password is incorrect"))
		return
	}

	tok := middleware.GetToken(c)
	tok.Set("user-id", usr.Id())
	tok.Set("exp", time.Now().Add(time.Hour*24*7))
	tok.Secret = org.SecretKey
	http.Render(c, 200, Token{tok.String()})
}

func create(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespace(c))
	usr := user.New(db)

	usrIn := &userIn{User: usr}

	// Decode response body to create new user
	if err := json.Decode(c.Request.Body, usrIn); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	if err := usr.GetByEmail(usr.Email); err == nil {
		http.Fail(c, 400, "Email is in use", errors.New("Email is in use"))
		return
	}

	if strings.Contains(usr.Email, "@") &&
		strings.Contains(usr.Email, ".") &&
		strings.Index(usr.Email, "@") < strings.Index(usr.Email, ".") &&
		len(usr.Email) > 5 {
		http.Fail(c, 400, "Email is not valid", errors.New("Email is not valid"))
		return
	}

	// Check for required fields
	if usr.Email == "" {
		http.Fail(c, 400, "Email is required", errors.New("Email is required"))
		return
	}

	if len(usrIn.Password) < 6 {
		http.Fail(c, 400, "Password needs to be atleast 6 characters", errors.New("Password needs to be atleast 6 characters"))
		return
	}

	if usrIn.Password != usrIn.PasswordConfirm {
		http.Fail(c, 400, "Passwords need to match", errors.New("Passwords need to match"))
		return
	}

	if hash, err := password.Hash(usrIn.Password); err != nil {
		http.Fail(c, 400, "Failed to hash user password", err)
	} else {
		usr.PasswordHash = hash
	}

	if err := usr.Put(); err != nil {
		http.Fail(c, 400, "Failed to create user", err)
	}
}
