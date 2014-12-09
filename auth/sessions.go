package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"

	"crowdstart.io/util/log"
)

const secret = "askjaakjl12"
const sessionName = "logged-in-" + kind

var store = sessions.NewCookieStore([]byte(secret))

func SaveSession(c *gin.Context, session *sessions.Session) error {
	return session.Save(c.Request, c.Writer)
}

func Delete(c *gin.Context, key string) error {
	session, err := store.Get(c.Request, sessionName)
	if err != nil {
		return err
	}
	delete(session.Values, key)
	return SaveSession(c, session)
}

func Set(c *gin.Context, key, value string) error {
	session, err := store.Get(c.Request, sessionName)
	if err != nil {
		return err
	}
	session.Values[key] = value
	return SaveSession(c, session)
}

func Get(c *gin.Context, key string) (string, error) {
	session, err := store.Get(c.Request, sessionName)
	if err != nil {
		return "", err
	}

	value, ok := session.Values[key].(string)
	if !ok {
		err := KeyError{key}
		log.Debug(err)
		return "", err
	}
	return value, SaveSession(c, session)
}

func ClearSession(c *gin.Context) error {
	session, err := store.Get(c.Request, sessionName)
	if err != nil {
		return err
	}
	session.Options.MaxAge = -1
	return SaveSession(c, session)
}
