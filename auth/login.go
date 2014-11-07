package auth

import (
	"code.google.com/p/go.crypto/bcrypt"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	"log"
)

// const sessionName = "crowdstartLogin"
const secret = "askjaakjl12"

var store = sessions.NewCookieStore([]byte(secret))

const kind = "user"
const sessionName = "logged-in-" + kind
const loginKey = "login-key"

func IsLoggedIn(c *gin.Context) bool {
	value, err := GetSession(c, loginKey)
	return err == nil && value != ""
}

func ClearSession(c *gin.Context) error {
	session, err := store.Get(c.Request, sessionName)
	if err != nil {
		return err
	}
	session.Values[loginKey] = ""
	return session.Save(c.Request, c.Writer)
}

func SetSession(c *gin.Context, key, value string) error {
	session, err := store.Get(c.Request, sessionName)
	if err != nil {
		return err
	}
	session.Values[key] = value
	return session.Save(c.Request, c.Writer)
}

func GetSession(c *gin.Context, key string) (string, error) {
	session, err := store.Get(c.Request, sessionName)
	if err != nil {
		return "", err
	}
	return session.Values[key].(string), nil
}

func VerifyUser(c *gin.Context) error {
	f := new(models.LoginForm)
	err := f.Parse(c)

	if err != nil {
		return err
	}

	hash, err := f.PasswordHash()
	log.Println(string(hash))

	if err != nil {
		return err
	}

	db := datastore.New(c)
	user := new(models.User)

	if err := db.GetKey("user", f.Email, user); err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(f.Password)); err != nil {
		return err
	}

	SetSession(c, loginKey, "PLEASE WORK") // sets cookie
	return nil
}
