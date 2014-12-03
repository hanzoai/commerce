package facebook

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"code.google.com/p/goauth2/oauth"
	"github.com/gin-gonic/gin"
	fb "github.com/huandu/facebook"

	"crowdstart.io/auth"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/util/log"

	"appengine/urlfetch"
)

/*
The OAuth stuff in this package is modelled after platform/admin/stripe.go
*/

// state is an arbitrary string which should be sent in order
// to prevent CSRF.
// http://stackoverflow.com/a/22892986
const state = func() string {
	b := make([]rune, n)
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}()

const appId = ""

// Grab this from the config (depending on if in production or not).
const root = "localhost:8080"

// URL to Callback
const redirectUri = net.QueryEscape("http://" + root + "/auth/facebook")

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
		return
	}

	user = models.User{}
}

func Login(c *gin.Context) {
	url := fmt.Sprintf(
		"https://www.facebook.com/dialog/oauth?client_id=%s&redirect_uri=%s&state=%s&scope=%s,%s&response_type=%s",
		appId, redirectUri, state,
		"email", "public_profile",
		"token",
	)
	ctx := middleware.GetAppEngine(c)
	client := urlfetch.Client(ctx)

	loginReq, err := client.Do("GET", url)
	if err != nil {
		log.Panic("loginReq failed.", err)
	}
}
