package admin

import (
	"appengine/urlfetch"
	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/util/router"
	"crowdstart.io/util/template"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

type TokenData struct {
	Access_token           string
	Error                  string
	Error_description      string
	Livemode               bool
	Refresh_token          string
	Scope                  string
	Stripe_publishable_key string
	Stripe_user_id         string
	Token_type             string
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
		c.Redirect(301, "dashboard")
	})

	// Admin login
	admin.GET("/login", func(c *gin.Context) {
		template.Render(c, "adminlte/login.html")
	})

	admin.POST("/login", func(c *gin.Context) {
		// Actually use this.
		auth.VerifyUser(c, "admin")
		c.Redirect(301, "dashboard")
	})

	admin.GET("/logout", func(c *gin.Context) {
		c.Redirect(301, "/")
	})

	// Admin User Profile
	admin.GET("/profile", middleware.LoginRequired(), func(c *gin.Context) {
	})

	admin.POST("/profile", func(c *gin.Context) {
		c.Redirect(301, "profile")
	})

	// Admin Dashboard
	admin.GET("/dashboard", middleware.LoginRequired(), func(c *gin.Context) {
		template.Render(c, "adminlte/dashboard.html")
	})

	// Admin Payment Connectors
	admin.GET("/connect", func(c *gin.Context) {
		template.Render(c, "adminlte/connect.html", "clientid", config.Get().Stripe.ClientId)
	})

	// Stripe End Points
	admin.GET("/stripe/callback", func(c *gin.Context) {
		req := c.Request
		code := req.URL.Query().Get("code")
		errStr := req.URL.Query().Get("error")

		// Failed to get back authorization code from Stripe
		if errStr != "" {
			template.Render(c, "stripe/failure.html", "error", errStr)
			return
		}

		ctx := middleware.GetAppEngine(c)
		client := urlfetch.Client(ctx)

		data := url.Values{}
		data.Set("client_secret", config.Get().Stripe.APISecret)
		data.Add("code", code)
		data.Add("grant_type", "authorization_code")

		tokenReq, err := http.NewRequest("POST", "https://connect.stripe.com/oauth/token", strings.NewReader(data.Encode()))
		if err != nil {
			c.Fail(500, err)
			return
		}

		// try to post to OAuth API
		res, err := client.Do(tokenReq)
		defer res.Body.Close()
		if err != nil {
			c.Fail(500, err)
			return
		}

		// decode the json
		jsonBlob, err := ioutil.ReadAll(res.Body)
		if err != nil {
			c.Fail(500, err)
			return
		}

		token := new(TokenData)

		// try and extract the json struct
		if err := json.Unmarshal(jsonBlob, token); err != nil {
			c.Fail(500, err)
		}

		// Stripe returned an error
		if token.Error != "" {
			template.Render(c, "stripe/failure.html", "error", token.Error)
			return
		}

		// Success
		template.Render(c, "stripe/success.html", "token", token.Access_token)

		// Update the user
		campaign := new(models.Campaign)

		db := datastore.New(ctx)

		// Get user instance
		db.GetKey("campaign", "skully", campaign)

		// Update  stripe token
		campaign.StripeToken = token.Access_token

		// Update in datastore
		db.PutKey("campaign", "skully", campaign)
	})
}

func NewUser(c *gin.Context, f models.RegistrationForm) error {
	m := f.User
	db := datastore.New(c)
	q := db.Query("user").
		Filter("Email =", m.Email).
		Limit(1)

	var users [1]models.User
	_, err := q.GetAll(db.Context, &users)

	if err != nil {
		return err
	}

	m.PasswordHash, err = f.PasswordHash()

	if err != nil {
		return err
	}

	if len(users) == 1 {
		return errors.New("Email is already registered")
	} else {
		_, err := db.Put("user", m)
		return err
	}
}
