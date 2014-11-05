package auth

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/form"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

// const sessionName = "crowdstartLogin"
const secret = "askjaakjl12"

var store = sessions.NewCookieStore([]byte(secret))

func IsLoggedIn(c *gin.Context, kind string) bool {
	session, err := store.Get(c.Request, kind)

	if err != nil {
		return false
	}

	return session.Values["id"] != nil
}

func setSession(c *gin.Context, kind, id string) error {
	session, err := store.Get(c.Request, kind)
	if err != nil {
		return err
	}
	session.Values["id"] = id
	return session.Save(c.Request, c.Writer)
}

func GetId(c *gin.Context, kind string) (string, error) {
	session, err := store.Get(c.Request, kind)
	if err != nil {
		return "", err
	}
	return session.Values["id"].(string), nil
}

func VerifyUser(c *gin.Context, kind string) error {
	f := new(models.LoginForm)
	err := form.Parse(c, f)

	if err != nil {
		c.Fail(401, err)
		return err
	}

	hash, err := f.PasswordHash()
	if err != nil {
		c.Fail(401, err)
		return err
	}

	db := datastore.New(c)
	q := db.Query(kind).
		Filter("Email =", f.Email).
		Filter("PasswordHash =", hash).
		Limit(1)

	if kind == "admin" {
		var admins [1]models.Admin
		_, err = q.GetAll(db.Context, &admins)
		if err != nil {
			return err
		}

		if err == nil && len(admins) == 1 {
			setSession(c, admins[0].Id, "crowdstart_"+kind) // sets cookie
			return nil
		}
		return errors.New("Email/Password combo is invalid.")

	} else if kind == "user" {
		var admins [1]models.User
		_, err = q.GetAll(db.Context, &admins)
		if err != nil {
			return err
		}

		if err == nil && len(admins) == 1 {
			setSession(c, admins[0].Id, "crowdstart_"+kind)
			return nil
		}
		
		return errors.New("Email/password combination is invalid.")
	} else {
		return errors.New("Unknown kind")
	}
}
