package login

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/auth"
	"crowdstart.com/auth/password"
	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/models/user"
	"crowdstart.com/util/log"
	"crowdstart.com/util/template"
)

func loginUser(c *gin.Context) (*user.User, error) {
	// Parse login form
	f := new(LoginForm)
	if err := f.Parse(c); err != nil {
		return nil, err
	}

	db := datastore.New(c)

	// Get user from database
	u := user.New(db)
	if err := u.GetByEmail(f.Email); err != nil {
		return nil, err
	}

	// Compare form password with saved hash
	if !password.HashAndCompare(u.PasswordHash, f.Password) {
		return nil, ErrorPasswordMismatch
	}

	// Set the loginKey value to the user id
	auth.Login(c, u)

	return u, nil
}

// GET /login
func Login(c *gin.Context) {
	template.Render(c, "login/login.html")
}

// POST /login
func LoginSubmit(c *gin.Context) {
	if _, err := loginUser(c); err == nil {
		log.Debug("Success")
		c.Redirect(302, config.UrlFor("platform"))
	} else {
		log.Debug("Failure")
		log.Debug("%#v", err)
		template.Render(c, "login/login.html", "failed", true)
	}
}

// GET /logout
func Logout(c *gin.Context) {
	err := auth.Logout(c)
	if err != nil {
		log.Panic("Error while logging out \n%v", err)
	}
	c.Redirect(302, config.UrlFor("platform"))
}
