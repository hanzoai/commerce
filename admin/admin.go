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

	// Admin index
	admin.GET("/", func(c *gin.Context) {
		template.Render(c, "index.html")
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

			if resp, err := client.Do(tokenReq); err == nil {
				defer resp.Body.Close()
				if jsonBlob, err := ioutil.ReadAll(resp.Body); err == nil {
					token := &TokenData{}
					if err := json.Unmarshal(jsonBlob, &token); err == nil {
						if len(token.Error) == 0 {
							template.Render(c, "stripe/success.html", "token", token.Access_token)
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
