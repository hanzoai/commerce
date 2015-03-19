package session

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"

	"crowdstart.io/config"
	"crowdstart.io/util/log"
)

var store = sessions.NewCookieStore([]byte(config.Secret))

func init() {
	store.Options = &sessions.Options{
		Domain:   config.CookieDomain,
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
	}
}

func saveSession(c *gin.Context, session *sessions.Session) error {
	return session.Save(c.Request, c.Writer)
}

func Get(c *gin.Context, key string) (string, error) {
	session, err := store.Get(c.Request, config.SessionName)
	if err != nil {
		return "", err
	}

	value, ok := session.Values[key].(string)
	if !ok {
		err := KeyError{key}
		log.Debug(err)
		return "", err
	}
	return value, saveSession(c, session)
}

func Set(c *gin.Context, key, value string) error {
	session, err := store.Get(c.Request, config.SessionName)
	if err != nil {
		return err
	}
	session.Values[key] = value
	return saveSession(c, session)
}

func Delete(c *gin.Context, key string) error {
	session, err := store.Get(c.Request, config.SessionName)
	if err != nil {
		return err
	}
	delete(session.Values, key)
	return saveSession(c, session)
}

func Clear(c *gin.Context) error {
	session, err := store.Get(c.Request, config.SessionName)
	if err != nil {
		return err
	}
	session.Options.MaxAge = -1
	return saveSession(c, session)
}
