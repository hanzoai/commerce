package session

import (
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/zap-proto/fiber/v3/middleware/adaptor"
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/log"
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

// getSession decodes the cookie-backed session from the request. gorilla/sessions
// reads the session cookie off a net/http request, so we bridge the zip.Ctx to
// one via the fiber adaptor.
func getSession(c *zip.Ctx) (*sessions.Session, error) {
	req, err := adaptor.ConvertRequest(c.Fiber(), false)
	if err != nil {
		return nil, err
	}
	return store.Get(req, config.SessionName)
}

// saveSession writes the session cookie back onto the response. gorilla writes
// Set-Cookie to a net/http ResponseWriter, so we run the save through the fiber
// adaptor which copies that response (Set-Cookie included) onto the zip.Ctx.
func saveSession(c *zip.Ctx, session *sessions.Session) error {
	var saveErr error
	h := adaptor.HTTPHandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		saveErr = session.Save(req, w)
	})
	if err := h(c.Fiber()); err != nil {
		return err
	}
	return saveErr
}

func Get(c *zip.Ctx, key string) (interface{}, error) {
	session, err := getSession(c)
	if err != nil {
		return nil, err
	}

	value, ok := session.Values[key]
	if !ok {
		return nil, err
	}
	return value, nil
}

func GetString(c *zip.Ctx, key string) string {
	session, err := getSession(c)
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

func MustGet(c *zip.Ctx, key string) interface{} {
	value, err := Get(c, key)
	if err != nil {
		panic(err)
	}

	return value
}

func Set(c *zip.Ctx, key, value string) error {
	session, err := getSession(c)
	if err != nil {
		return err
	}
	session.Values[key] = value
	return saveSession(c, session)
}

func Delete(c *zip.Ctx, key string) error {
	session, err := getSession(c)
	if err != nil {
		return err
	}
	delete(session.Values, key)
	return saveSession(c, session)
}

func Clear(c *zip.Ctx) error {
	session, err := getSession(c)
	if err != nil {
		return err
	}
	session.Options.MaxAge = -1
	return saveSession(c, session)
}
