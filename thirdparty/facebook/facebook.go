package facebook

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
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
var state = func() string {
	n := 16
	b := make([]rune, n)
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}()

const appId = ""

// Grab this from the config (depending on if in production or not).
const base = "localhost:8080"

// URL to Callback
var redirectUri = url.QueryEscape(base + "/auth/facebook")

const graphVersion = "v2.2"

// TODO Expose a route to this.
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

	resState := req.URL.Query().Get("state")
	if state != resState {
		log.Panic("CSRF attempt \n\tExpected: %s \nt\tActual: %s",
			state, resState)
	}

	ctx := middleware.GetAppEngine(c)
	client := urlfetch.Client(ctx)

	url := fmt.Sprintf("http://graph.facebook.com/%s/me&access_token=%s", graphVersion, accessToken)
	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		log.Panic("Error while creating a http request \n%v", err)
	}

	res, err := client.Do(req)
	if err != nil {
		log.Panic("Graph API not available", err)
	}

	jsonBlob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Panic("Error reading from Graph API", err)
	}

	user := new(models.User)
	err = json.Unmarshal(jsonBlob, &user.Facebook)
	if err != nil {
		log.Panic("Error parsing Graph API JSON response", err)
	}

	db := datastore.New(ctx)

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

	err = auth.Login(c, user.Email)
	if err != nil {
		log.Panic("Error while setting session", err)
	}

	c.Redirect(301, "/")
}

func LoginUser(c *gin.Context) {
	url := fmt.Sprintf(
		"https://www.facebook.com/dialog/oauth?client_id=%s&redirect_uri=%s&state=%s&scope=%s,%s&response_type=%s",
		appId, redirectUri, state,
		"email", "public_profile",
		"token",
	)
	ctx := middleware.GetAppEngine(c)
	client := urlfetch.Client(ctx)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Panic("Error while creating a http request \n%v", err)
	}

	res, err := client.Do(req)
	defer res.Body.Close()
	if err != nil {
		log.Panic("loginReq failed.", err)
	}
}

func IsAccessTokenExpired(c *gin.Context) bool {
	user := auth.GetUser(c)
	if user == nil {
		return true
	}

	if user.Facebook.AccessToken == "" {
		return true
	}

	ctx := middleware.GetAppEngine(c)
	client := urlfetch.Client(ctx)

	url := fmt.Sprintf("http://graph.facebook.com/v2.2/me/permission?access_token=%s", user.Facebook.AccessToken)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Panic("Error while creating an http request \n%v", err)
	}

	res, err := client.Do(req)
	defer res.Body.Close()
	if err != nil {
		log.Panic("Checking permissions using the Graph API failed", err)
	}

	jsonBlob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Panic("Error reading the permissions response", err)
	}

	j := string(jsonBlob)

	return strings.Contains(j, "public_profile")
}
