package auth

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	"log"
)

// const sessionName = "crowdstartLogin"
const secret = "askjaakjl12"

var store = sessions.NewCookieStore([]byte(secret))

const kind = "user"
const sessionName = "logged-in-"+kind

func IsLoggedIn(c *gin.Context) bool {
	session, err := store.Get(c.Request, sessionName)

	if err != nil {
		return false
	}

	log.Println(session.Values)
	return session.Values["key"] != nil
}

func setSession(c *gin.Context, key string) error {
	session, err := store.Get(c.Request, sessionName)
	if err != nil {
		return err
	}
	session.Values["key"] = key
	return session.Save(c.Request, c.Writer)
}

func GetKey(c *gin.Context) (string, error) {
	session, err := store.Get(c.Request, sessionName)
	if err != nil {
		return "", err
	}
	return session.Values["key"].(string), nil
}

func VerifyUser(c *gin.Context) error {
	f := new(models.LoginForm)
	err := f.Parse(c)
	
	if err != nil {
		return err
	}

	hash := f.PasswordHash()

	log.Println(string(hash))
	
	db := datastore.New(c)
	q := db.Query(kind).
		Filter("Email =", f.Email).
		Filter("PasswordHash =", hash).
		KeysOnly().
		Limit(1)

	keys, err := q.GetAll(db.Context, nil)
	if err != nil {
		return err
	}

	log.Printf("keys %d", len(keys))
	if len(keys) == 1 {
		setSession(c, keys[0].StringID()) // sets cookie
		return nil
	}
	return errors.New("Email/password combination is invalid.")
}
