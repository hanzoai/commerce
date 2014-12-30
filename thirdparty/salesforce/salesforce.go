package salesforce

import (
	"encoding/json"
	"errors"
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
	Tokens   SalesforceTokens
	JsonBlob string
}

type SalesforceTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	InstanceUrl  string `json:"instance_url"`
	Id           string `json:"id"`
	IssuedAt     string `json:"issued_at"`
	Signature    string `json:"signature"`

	ErrorDescription string `json:"error_description"`
	Error            string `json:"error"`
}

type DescribeResponse struct {
	SObjects     string `json:"sobjects"`
	Connect      string `json:"connect"`
	Query        string `json:"query"`
	Theme        string `json:"theme"`
	QueryAll     string `json:"queryAll"`
	Tooling      string `json:"tooling"`
	Chatter      string `json:"chatter"`
	Analytics    string `json:"analytics"`
	Recent       string `json:"recent"`
	Commerce     string `json:"commerce"`
	Licensing    string `json:"licensing"`
	Identity     string `json:"identity"`
	FlexiPage    string `json:"flexiPage"`
	Search       string `json:"search"`
	QuickActions string `json:"quickActions"`
	AppMenu      string `json:"appMenu"`

	Message   string `json:"message"`
	ErrorCode string `json:"errorCode"`
}

func (a *Api) request(method, url string, params *url.Values) (*http.Request, error) {
	req, err := http.NewRequest(method, url, strings.NewReader(params.Encode()))
	if err != nil {
		log.Error("Could not create request: %v", err)
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+a.Tokens.AccessToken)

	return req, err
}

func (a *Api) get(url string, params *url.Values) (*http.Request, error) {
	req, err := a.request("GET", url, params)
	return req, err
}

func getClient(c *gin.Context) *http.Client {
	ctx := middleware.GetAppEngine(c)
	client := urlfetch.Client(ctx)

	return client
}

func Init(c *gin.Context, accessToken, refreshToken, instanceUrl, id, issuedAt, signature string) (*Api, error) {
	client := getClient(c)

	api := &(Api{
		Tokens: SalesforceTokens{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			InstanceUrl:  instanceUrl,
			Id:           id,
			IssuedAt:     issuedAt,
			Signature:    signature}})

	response := make([]DescribeResponse, 0, 0)

	Describe(api, client, &response)

	if len(response) == 0 || response[0].ErrorCode != "" {
		if err := Refresh(c, refreshToken, &api.Tokens); err != nil {
			return nil, err
		}

		err := Describe(api, client, &response)
		if err != nil {
			return nil, err
		}

		if len(response) == 0 || response[0].ErrorCode != "" {
			return nil, errors.New("Nothing to decode")
		}
	}

	return api, nil
}

func Describe(api *Api, client *http.Client, response *[]DescribeResponse) error {
	params := url.Values{}
	req, err := api.get(api.Tokens.InstanceUrl+DescribePath, &params)

	if err != nil {
		return err
	}

	res, err := client.Do(req)
	defer res.Body.Close()

	if err != nil {
		return err
	}

	jsonBlob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	api.JsonBlob = string(jsonBlob[:])

	if err := json.Unmarshal(jsonBlob, response); err != nil {
		return err
	}

	return nil
}

func Refresh(c *gin.Context, refreshToken string, tokens *SalesforceTokens) error {
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
		return err
	}

	log.Debug("Trying to send request %v with params %v", tokenReq, data)

	// try to post to refresh token API
	res, err := client.Do(tokenReq)
	defer res.Body.Close()
	if err != nil {
		log.Error("Could not post a Refresh Token: %v", err)
		return err
	}

	// decode the json
	jsonBlob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Error("Could not decode jsonblob: %v", err)
		return err
	}

	log.Debug("Trying to unmarshal jsonBlob: %s", jsonBlob)

	// try and extract the json struct
	if err := json.Unmarshal(jsonBlob, tokens); err != nil {
		log.Error("Could not unmarshal jsonBlob: %v", err)
		return err
	}

	if tokens.Error != "" {
		return errors.New(tokens.ErrorDescription)
	}

	log.Debug("New Access Token:%v", tokens.AccessToken)

	return nil
}
