package salesforce

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"crowdstart.io/config"
	"crowdstart.io/middleware"
	"crowdstart.io/util/log"

	"github.com/gin-gonic/gin"

	"appengine/urlfetch"
)

var LoginUrl = "https://login.salesforce.com/services/oauth2/token"
var DescribePath = "/services/data/v29.0/"

type Api struct {
	Tokens SalesforceTokens
}

type SalesforceTokens struct {
	AccessToken string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	InstanceUrl string `json:"instance_url"`
	Id          string `json:"id"`
	IssuedAt    string `json:"issued_at"`
	Signature   string `json:"signature"`
}

func (a *Api) request(method, url string, params *url.Values) (*http.Request, error){
	req, err := http.NewRequest(method, url, strings.NewReader(params.Encode()))
	if err != nil {
		log.Error("Could not create request: %v", err)
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer " + a.Tokens.AccessToken)

	return req, err
}

func (a *Api) get(url string, params *url.Values) (*http.Request, error){
	req, err := a.request("GET", url, params)
	return req, err
}

func getClient(c *gin.Context) *http.Client {
	ctx := middleware.GetAppEngine(c)
	client := urlfetch.Client(ctx)

	return client
}

func Init(c *gin.Context, accessToken, refreshToken, instanceUrl, id, issuedAt, signature string) (*Api, error) {
	ctx := middleware.GetAppEngine(c)
	client := urlfetch.Client(ctx)

	api := &(Api{
		Tokens: SalesforceTokens{
			AccessToken: accessToken,
			RefreshToken: refreshToken,
			InstanceUrl: instanceUrl,
			Id: id,
			IssuedAt: issuedAt,
			Signature: signature}})

	params := url.Values{}
	if req, err := api.get(api.Tokens.InstanceUrl + DescribePath, params); err != nil {

	}

	return api, nil
}

func Refresh(c *gin.Context, refreshToken string) (*SalesforceTokens, error) {
	ctx := middleware.GetAppEngine(c)
	client := urlfetch.Client(ctx)

	// https://help.salesforce.com/HTViewHelpDoc?id=remoteaccess_oauth_refresh_token_flow.htm&language=en_US
	data := url.Values{}
	data.Add("grant_type", "refresh_token")
	data.Set("client_id", config.Salesforce.ConsumerKey)
	data.Set("client_secret", config.Salesforce.ConsumerSecret)
	data.Set("refresh_token", refreshToken)

	tokenReq, err := http.NewRequest("POST", LoginUrl, strings.NewReader(data.Encode()))
	if err != nil {
		log.Error("Could not create request: %v", err)
		return nil, err
	}

	log.Debug("Trying to send request %v with params %v", tokenReq, data)

	// try to post to refresh token API
	res, err := client.Do(tokenReq)
	defer res.Body.Close()
	if err != nil {
		log.Error("Could not post a Refresh Token: %v", err)
		return nil, err
	}

	// decode the json
	jsonBlob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Error("Could not decode jsonblob: %v", err)
		return nil, err
	}

	token := new(SalesforceTokens)
	log.Debug("Trying to unmarshal jsonBlob: %s", jsonBlob)

	// try and extract the json struct
	if err := json.Unmarshal(jsonBlob, token); err != nil {
		log.Error("Could not unmarshal jsonBlob: %v", err)
		return nil, err
	}

	log.Debug("New Access Token:%v", token.AccessToken)

	return token, nil
}

