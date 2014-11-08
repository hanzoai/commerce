package auth

import (
	"github.com/gorilla/sessions"
	"github.com/gin-gonic/gin"
)

const secret = "askjaakjl12"
const sessionName = "logged-in-" + kind

var store = sessions.NewCookieStore([]byte(secret))

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
