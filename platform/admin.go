package admin

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

func adminIndex(c *gin.Context) {
	template.Render(c, "index.html")
}

// Admin Register
func adminRegister(c *gin.Context) {
	template.Render(c, "adminlte/register.html")
}

func adminSubmitRegister(c *gin.Context) {
	c.Redirect(301, "dashboard")
}

// Admin login
func adminLogin(c *gin.Context) {
	template.Render(c, "adminlte/login.html")
}

func adminSubmitLogin(c *gin.Context) {
	if err := auth.VerifyUser(c); err == nil {
		log.Println("Success")
		c.Redirect(301, "dashboard")
	} else {
		log.Println("Failure")
		log.Printf("%#v", err)
		c.Redirect(301, "login")
	}
}

func adminLogout(c *gin.Context) {
	auth.ClearSession(c)
	c.Redirect(301, "/")
}

// Admin User Profile
func adminProfile(c *gin.Context) {
}

func adminSubmitProfile(c *gin.Context) {
	c.Redirect(301, "profile")
}

// Admin Dashboard
func adminDashboard(c *gin.Context) {
	template.Render(c, "adminlte/dashboard.html")
}

// Admin Payment Connectors
func adminConnect(c *gin.Context) {
	template.Render(c, "adminlte/connect.html", "clientid", config.Get().Stripe.ClientId)
}

// Stripe End Points
func stripeCallback(c *gin.Context) {
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
		template.Render(c, "adminlte/connect.html", "error", token.Error, "clientid", config.Get().Stripe.ClientId)
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
}
