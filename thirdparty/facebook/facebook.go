package facebook

import (
	"fmt"
	"net/url"

	"github.com/gin-gonic/gin"
	fb "github.com/huandu/facebook"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/log"

	"appengine"
	"appengine/urlfetch"
)

/*
The OAuth stuff in this package is modelled after platform/admin/stripe.go
*/

// state is an arbitrary string which should be sent in order
// to prevent CSRF.
// http://stackoverflow.com/a/22892986
// Issues may arise when multiple instances are deployed.
/*var state = func() string {
	n := 16
	b := make([]rune, n)
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}()*/

// TODO Create a way to change state without invalidating previous CSRF tokens
// Possibly TTL kv store?
const state = "a17381zxncm,nzxcm, -SADs;d'asd2aj~^&*^!@*&%#!^ajkdhas"

const appId = "739937846096416"

// TODO Grab this from the config (depending on if in production or not).
const base = "24.79.105.138:8080/store"

// URL to Callback
// TODO use UrlFor
var redirectUri = url.QueryEscape(base + "/auth/facebook")

const graphVersion = "v2.2"

var app *fb.App

func newSession(c *gin.Context, accessToken string) *fb.Session {
	if app == nil {
		app = fb.New(appId, "eb737a205043f4cc73b2e7107c162a36")
		app.RedirectUri = "localhost" // Not useful yet
	}
	session := app.Session(accessToken)
	session.HttpClient = urlfetch.Client(appengine.NewContext(c.Request))
	return session
}

// GET /auth/callback
func Callback(c *gin.Context) {
	req := c.Request
	e := req.URL.Query().Get("error")
	if e != "" {
		description := req.URL.Query().Get("error_description")
		reason := req.URL.Query().Get("error_reason")
		log.Info(
			"Error in facebook callback \n %s \n %s \n %s",
			e, reason, description,
		)
		return
	}

	accessToken := req.URL.Query().Get("access_token")
	if accessToken == "" {
		log.Panic("There is no access token")
	}

	if resState := req.URL.Query().Get("state"); state != resState {
		log.Panic("CSRF attempt \n\tExpected: %s \nt\tActual: %s",
			state, resState)
	}

	session := newSession(c, accessToken)
	if err := session.Validate(); err != nil {
		log.Panic("AccessToken is invalid %s", session.AccessToken())
	}

	me, err := session.Get("/me", nil)
	if err != nil {
		log.Panic("Error accessing Graph API with accessToken", err)
	}

	user := new(models.User)
	if err := me.Decode(&user.Facebook); err != nil {
		log.Panic("Error parsing /me response", err)
	}

	db := datastore.New(c)

	existingUser := new(models.User)
	db.GetKey("user", user.Email, existingUser)

	if existingUser == nil {
		existingUser = user
	}
	existingUser.FirstName = user.Facebook.FirstName
	existingUser.LastName = user.Facebook.LastName
	existingUser.Email = user.Facebook.Email

	_, err = db.PutKey("user", existingUser.Email, user)
	if err != nil {
		log.Panic("Error creating user using Facebook", err)
	}

	c.String(200, "Request processed")
}

// GET /auth
func LoginUser(c *gin.Context) {
	url := fmt.Sprintf(
		"https://www.facebook.com/dialog/oauth?client_id=%s&redirect_uri=%s&state=%s&scope=%s,%s&response_type=%s",
		appId, redirectUri, state,
		"email", "public_profile",
		"token",
	)

	if auth.IsLoggedIn(c) {
		user, _ := auth.GetUser(c)
		if user.Facebook.AccessToken != "" {
			session := newSession(c, user.Facebook.AccessToken)
			if err := session.Validate(); err == nil {
				// Token is still valid
				auth.Login(c, user.Email)
				c.Redirect(301, "/profile")
				return
			}
		}
	}

	c.Redirect(301, url)
}
