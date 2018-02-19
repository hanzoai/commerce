package session

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"

	"hanzo.io/config"
	"hanzo.io/log"
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

func saveSession(c *context.Context, session *sessions.Session) error {
	return session.Save(c.Request, c.Writer)
}

func Get(c *context.Context, key string) (interface{}, error) {
	session, err := store.Get(c.Request, config.SessionName)
	if err != nil {
		return nil, err
	}

	value, ok := session.Values[key]
	if !ok {
		return nil, err
	}
	return value, nil
}

func GetString(c *context.Context, key string) string {
	session, err := store.Get(c.Request, config.SessionName)
	if err != nil {
		log.Warn(err)
		return ""
	}

	// Check for value in session
	value, ok := session.Values[key]
	if !ok {
		return ""
	}

	// Coerce to string
	str, _ := value.(string)

	return str
}

func MustGet(c *context.Context, key string) interface{} {
	value, err := Get(c, key)
	if err != nil {
		panic(err)
	}

	return value
}

func Set(c *context.Context, key, value string) error {
	session, err := store.Get(c.Request, config.SessionName)
	if err != nil {
		return err
	}
	session.Values[key] = value
	return saveSession(c, session)
}

func Delete(c *context.Context, key string) error {
	session, err := store.Get(c.Request, config.SessionName)
	if err != nil {
		return err
	}
	delete(session.Values, key)
	return saveSession(c, session)
}

func Clear(c *context.Context) error {
	session, err := store.Get(c.Request, config.SessionName)
	if err != nil {
		return err
	}
	session.Options.MaxAge = -1
	return saveSession(c, session)
}
