package admin

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/thirdparty/salesforce"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"

	"github.com/gin-gonic/gin"

	"appengine/urlfetch"
)

// Salesforce End Points
func SalesforceCallback(c *gin.Context) {
	req := c.Request
	code := req.URL.Query().Get("code")

	ctx := middleware.GetAppEngine(c)
	client := urlfetch.Client(ctx)

	// http://www.salesforce.com/us/developer/docs/api_rest/index_Left.htm#StartTopic=Content/quickstart.htm
	// Below one is the secret good documentation
	// https://www.salesforce.com/us/developer/docs/api_rest/Content/intro_understanding_web_server_oauth_flow.htm
	data := url.Values{}
	data.Add("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", config.Salesforce.ConsumerKey)
	data.Set("client_secret", config.Salesforce.ConsumerSecret)
	data.Set("redirect_uri", config.Salesforce.CallbackURL)

	tokenReq, err := http.NewRequest("POST", salesforce.LoginUrl, strings.NewReader(data.Encode()))
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

	token := new(salesforce.SalesforceTokens)

	// try and extract the json struct
	if err := json.Unmarshal(jsonBlob, token); err != nil {
		c.Fail(500, err)
		return
	}

	// Salesforce does not Error ;)
	// if token.Error != "" {
	// 	template.Render(c, "adminlte/connect.html", "error", token.Error)
	// 	return
	// }

	// Update the user
	campaign := new(models.Campaign)

	db := datastore.New(ctx)

	// Get user
	email, err := auth.GetEmail(c)
	if err != nil {
		log.Panic("Unable to get email from session: %v", err)
	}

	// Get user instance
	if err := db.GetKey("campaign", email, campaign); err != nil {
		log.Panic("Unable to get campaign from database: %v", err)
	}

	// Update Salesforce data
	campaign.Salesforce.AccessToken = token.AccessToken
	campaign.Salesforce.RefreshToken = token.RefreshToken
	campaign.Salesforce.InstanceUrl = token.InstanceUrl
	campaign.Salesforce.Id = token.Id
	campaign.Salesforce.IssuedAt = token.IssuedAt
	campaign.Salesforce.Signature = token.Signature

	// Update in datastore
	if _, err := db.PutKey("campaign", email, campaign); err != nil {
		log.Panic("Failed to update campaign: %v", err)
	}

	// Success
	template.Render(c, "salesforce/success.html", "token", token.AccessToken)
}

func TestSalesforceConnection(c *gin.Context) {
	// Get user
	email, err := auth.GetEmail(c)
	if err != nil {
		log.Panic("Unable to get email from session: %v", err)
	}

	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)

	campaign := new(models.Campaign)

	// Get user instance
	if err := db.GetKey("campaign", email, campaign); err != nil {
		log.Panic("Unable to get campaign from database: %v", err)
	}

	api, err := salesforce.Init(
		c,
		campaign.Salesforce.AccessToken,
		campaign.Salesforce.RefreshToken,
		campaign.Salesforce.InstanceUrl,
		campaign.Salesforce.Id,
		campaign.Salesforce.IssuedAt,
		campaign.Salesforce.Signature)

	if err != nil {
		log.Panic("Unable to log in: %v", err)
		return
	}

	log.Info("YAY %v", api.JsonBlob)
}
