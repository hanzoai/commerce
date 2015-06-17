package account

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/auth/password"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/user"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
)

type loginRes struct {
	Token string `json:"token"`
}

type loginReq struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"passwordConfirm"`
}

func get(c *gin.Context) {
	usr := middleware.GetUser(c)

	if err := usr.LoadReferrals(); err != nil {
		http.Fail(c, 500, "User referral data could get be queried", err)
		return
	}

	if err := usr.LoadOrders(); err != nil {
		http.Fail(c, 500, "User order data could get be queried", err)
		return
	}

	if err := usr.CalculateBalances(); err != nil {
		http.Fail(c, 500, "User balance data could get be queried", err)
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

func login(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespace(c))

	req := &loginReq{}

	// Decode response body to create new user
	if err := json.Decode(c.Request.Body, req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Get user by email
	usr := user.New(db)
	if err := usr.GetByEmail(req.Email); err != nil {
		http.Fail(c, 401, "Email or password is incorrect", errors.New("Email or password is incorrect"))
		return
	}

	// Check user's password
	if !password.HashAndCompare(usr.PasswordHash, req.Password) {
		http.Fail(c, 401, "Email or password is incorrect", errors.New("Email or password is incorrect"))
		return
	}

	// If user is not enabled fail
	if !usr.Enabled {
		http.Fail(c, 401, "User is not enabled", errors.New("User is not enabled"))
		return
	}

	// Return a new token with user id set
	tok := middleware.GetToken(c)
	tok.Set("user-id", usr.Id())
	tok.Set("exp", time.Now().Add(time.Hour*24*7))

	http.Render(c, 200, loginRes{tok.String()})
}

func exists(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespace(c))
	usr := user.New(db)

	query := c.Request.URL.Query()
	email := query.Get("email")

	if err := usr.GetByEmail(email); err == nil {
		http.Fail(c, 400, "Email is in use", errors.New("Email is in use"))
		return
	}

	http.Render(c, 200, gin.H{"status": "ok"})
}
