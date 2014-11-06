package admin

import (
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/router"
	"crowdstart.io/util/template"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

type TokenData struct {
	Livemode                                                                                                         bool
	Token_type, Stripe_publishable_key, Scope, Stripe_user_id, Refresh_token, Access_token, Error, Error_description string
}

func init() {
	admin := router.New("/admin/")

	// Admin Index
	admin.GET("/", func(c *gin.Context) {
		template.Render(c, "index.html")
	})

	// Admin Register
	admin.GET("/register", func(c *gin.Context) {
		template.Render(c, "adminlte/register.html")
	})

	admin.POST("/register", func(c *gin.Context) {
	})

	// Admin login
	admin.GET("/login", func(c *gin.Context) {
		template.Render(c, "adminlte/login.html")
	})

	admin.POST("/login", func(c *gin.Context) {
	})

	// Show stripe button
	admin.GET("/stripe/connect", func(c *gin.Context) {
		template.Render(c, "stripe/connect.html", "clientid", config.Get().Stripe.ClientId)
	})

	admin.GET("/stripe/callback", func(c *gin.Context) {
		req := c.Request
		error := req.URL.Query().Get("error")
		code := req.URL.Query().Get("code")

		if len(error) == 0 {

			transport := http.Transport{}

			client := &http.Client{
				Transport: &transport,
			}

			data := url.Values{}
			data.Set("client_secret", config.Get().Stripe.APISecret)
			data.Add("code", code)
			data.Add("grant_type", "authorization_code")

			tokenReq, _ := http.NewRequest("POST", "https://connect.stripe.com/oauth/token", strings.NewReader(data.Encode()))

			// try to post to OAuth API
			if resp, err := client.Do(tokenReq); err == nil {
				defer resp.Body.Close()
				// decode the json
				if jsonBlob, err := ioutil.ReadAll(resp.Body); err == nil {
					token := &TokenData{}
					// try and extract the json struct
					if err := json.Unmarshal(jsonBlob, &token); err == nil {
						if len(token.Error) == 0 {
							// success!, render the template
							template.Render(c, "stripe/success.html", "token", token.Access_token)

							// update the user
							user := &models.User{}
							db := datastore.New(c)

							// get user instance
							db.GetKey("user", "admin", user)

							// update  stripe token
							user.StripeToken = token.Access_token

							// update in datastore
							db.PutKey("user", "admin", user)

							// Everything below is Error Handling
						} else {
							error = token.Error
						}
					} else {
						error = err.Error()
					}
				} else {
					error = err.Error()
				}
			} else {
				error = err.Error()
			}
		}

		if len(error) > 0 {
			template.Render(c, "stripe/failure.html", "error", error)
		}
	})

	// Redirected on success from connect button.
	admin.POST("/stripe/success/:userid/:token", func(c *gin.Context) {
		db := datastore.New(c)
		token := c.Params.ByName("token")
		userid := c.Params.ByName("userid")

		// get user instance
		user := new(models.User)
		db.GetKey("user", userid, user)

		// update  stripe token
		user.StripeToken = token

		// update in datastore
		db.PutKey("user", userid, user)

		template.Render(c, "stripe/success.html")
	})
}
