package salesforce

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"crowdstart.io/config"
	"crowdstart.io/middleware"

	"github.com/davidtai/go-force/force"
	"github.com/gin-gonic/gin"

	"appengine/urlfetch"
)

type Api struct {
	api *force.ForceApi
}

type refreshedSalesforceToken struct {
	AccessToken string `json:"access_token"`
	InstanceUrl string `json:"instance_url"`
	Id          string `json:"id"`
	IssuedAt    string `json:"issued_at"`
	Signature   string `json:"signature"`
}

func Init(c *gin.Context, accessToken, refreshToken, instanceUrl, id, issuedAt, signature string) (*Api, error) {
	api, err := force.Set(accessToken, instanceUrl, id, issuedAt, signature)
	// attempt to reauthenticate using refresh token
	if err != nil {
		var refreshedToken *refreshedSalesforceToken
		if refreshedToken, err = Refresh(c, refreshToken); err != nil {
			return nil, err
		}

		if api, err = force.Set(refreshedToken.AccessToken, refreshedToken.InstanceUrl, refreshedToken.Id, refreshedToken.IssuedAt, refreshedToken.Signature); err != nil {
			return nil, err
		}
	}
	return &Api{api: api}, nil
}

func Refresh(c *gin.Context, refreshToken string) (*refreshedSalesforceToken, error) {
	ctx := middleware.GetAppEngine(c)
	client := urlfetch.Client(ctx)

	// https://help.salesforce.com/HTViewHelpDoc?id=remoteaccess_oauth_refresh_token_flow.htm&language=en_US
	data := url.Values{}
	data.Add("refresh_token", refreshToken)
	data.Add("grant_type", "refresh_token")
	data.Set("client_id", config.Salesforce.ConsumerKey)
	data.Set("client_secret", config.Salesforce.ConsumerSecret)

	tokenReq, err := http.NewRequest("POST", "https://login.salesforce.com/", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	// try to post to refresh token API
	res, err := client.Do(tokenReq)
	defer res.Body.Close()
	if err != nil {
		return nil, err
	}

	// decode the json
	jsonBlob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	token := new(refreshedSalesforceToken)

	// try and extract the json struct
	if err := json.Unmarshal(jsonBlob, token); err != nil {
		return nil, err
	}

	// try and extract the json struct
	if err := json.Unmarshal(jsonBlob, token); err != nil {
		return nil, err
	}

	return token, nil
}

func (a *Api) GetSObject(id string, out force.SObject) error {
	return a.api.GetSObject(id, out)
}
