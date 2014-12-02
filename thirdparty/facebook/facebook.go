package facebook

import (
	"errors"
	"fmt"
	"net/http"

	"code.google.com/p/goauth2/oauth"
	"github.com/gin-gonic/gin"

	"crowdstart.io/middleware"
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

func Login(c *gin.Context) {
	redirectUri := ""
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
