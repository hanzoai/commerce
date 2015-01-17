package admin

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

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

	token := new(salesforce.SalesforceTokenResponse)

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
		log.Panic("Unable to get email from session: %v", err, c)
	}

	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)

	campaign := new(models.Campaign)

	// Get user instance
	if err = db.GetKey("campaign", email, campaign); err != nil {
		log.Panic("Unable to get campaign from database: %v", err, c)
	}

	// Test Connecting to Salesforce
	client := salesforce.New(c, campaign, true)

	describeResponse := new(salesforce.DescribeResponse)
	if err = client.Describe(describeResponse); err != nil {
		log.Panic("Describe Failed %v, %v", err, string(client.LastBody[:]), c)
	}
	log.Info("Describe Success %v", describeResponse, c)

	// Test Upsert
	// Please don't actually mail anything to this
	user := models.User{
		Id:        "TestId",
		FirstName: "Test User",
		LastName:  "Please do not mail anything to this test user.",
		Phone:     "555-5555",
		Email:     "TestUser@verus.com",
		ShippingAddress: models.Address{
			Line1:      "1600 Pennsylvania Avenue",
			Line2:      "Suite 202",
			City:       "Northwest",
			State:      "District of Columbia",
			PostalCode: "20500",
			Country:    "United States",
		},
	}

	if err = client.Push(&user); err != nil {
		log.Panic("Upsert Failed: %v, %v", err, string(client.LastBody[:]), c)
	}
	log.Info("Upsert Success %v", user, c)

	// Test GET Query using Email
	user2 := models.User{}
	if err = client.Pull(user.Id, &user2); err != nil {
		log.Panic("Get Failed: %v, %v", err, string(client.LastBody[:]), c)
	}
	log.Info("Get Success %v", user2, c)

	now := time.Now()

	// Test to see if salesforce reports back that we upserted a user
	ids := new([]string)
	if err = client.GetUpdatedContacts(now.Add(-15*time.Minute), now, ids); err != nil {
		log.Panic("Getting Updated Contacts Failed: %v, %v", err, string(client.LastBody[:]), c)
	}
	log.Info("Get Updated Contacts Success %v", ids, c)

	c.String(200, "Success!")
}
