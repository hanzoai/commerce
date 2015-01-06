package salesforce

import (
	"encoding/json"
	"errors"
	"fmt"
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

// Api Data Container
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

type Api struct {
	Tokens       SalesforceTokens
	LastQuery    *http.Request
	LastJsonBlob string
	Client       *http.Client
}

func (a *Api) request(method, url string, data string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, strings.NewReader(data))

	if err != nil {
		log.Error("Could not create request: %v", err)
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.Tokens.AccessToken)
	a.LastQuery = req

	return req, err
}

// Get the HttpClient from the Gin context
func getClient(c *gin.Context) *http.Client {
	ctx := middleware.GetAppEngine(c)
	client := urlfetch.Client(ctx)

	return client
}

func Init(c *gin.Context, accessToken, refreshToken, instanceUrl, id, issuedAt, signature string) (*Api, error) {
	// Load Data into API
	api := &(Api{
		Tokens: SalesforceTokens{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			InstanceUrl:  instanceUrl,
			Id:           id,
			IssuedAt:     issuedAt,
			Signature:    signature},
		Client: getClient(c)})

	// Hit the topmost RESTful endpoint to test if credentials work
	response := make([]DescribeResponse, 1, 1)

	if err := Describe(api, response); err != nil {
		return nil, err
	}

	// If the endpoint has and error, try again after refreshing credentials
	if len(response) == 0 || response[0].ErrorCode != "" {
		// Try to get new API tokens by using the refresh token
		if err := Refresh(c, refreshToken, &api.Tokens); err != nil {
			return nil, err
		}

		// Try to hit the endpoint again
		err := Describe(api, response)
		if err != nil {
			return nil, err
		}

		if len(response) == 0 || response[0].ErrorCode != "" {
			return nil, errors.New("Nothing to decode")
		}
	}

	return api, nil
}

func request(api *Api, method, path string, headers map[string]string, data string) ([]byte, error) {
	client := api.Client

	req, err := api.request(method, api.Tokens.InstanceUrl+path, data)

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	if err != nil {
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	jsonBlob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	api.LastJsonBlob = string(jsonBlob[:])

	return jsonBlob, nil
}

func UpsertContact(api *Api, contact *Contact) error {
	if contact.Email == "" {
		errors.New("Email is required")
	}

	contactBytes, err := json.Marshal(contact)
	if err != nil {
		return err
	}

	contactJSON := string(contactBytes[:])

	// strings.Replace required to bypass broken Salesforce period parsing
	path := fmt.Sprintf(ContactUpsertUsingEmailPath, strings.Replace(contact.Email, ".", "_", -1))

	jsonBlob, err := request(api, "PATCH", path, map[string]string{"Content-Type": "application/json"}, contactJSON)
	if err != nil {
		return err
	}

	api.LastJsonBlob = string(jsonBlob[:])

	response := UpsertResponse{}

	if err := json.Unmarshal(jsonBlob, &response); err != nil {
		return err
	}

	if !response.Success {
		return errors.New(response.Errors[0].Message)
	}

	return nil
}

func GetContactByEmail(api *Api, email string) ([]Contact, error) {
	// Not the safest thing in the world
	path := ContactQueryPath + "%27" + email + "%27"

	jsonBlob, err := request(api, "GET", path, map[string]string{}, "")
	if err != nil {
		return nil, err
	}

	queryResponse := new(QueryResponse)

	if err := json.Unmarshal(jsonBlob, queryResponse); err != nil {
		return nil, err
	}

	length := len(queryResponse.Records)

	response := make([]Contact, length, length)
	for i, record := range queryResponse.Records {
		jsonBlob, err = request(api, "GET", record.Attributes.Url, map[string]string{}, "")
		if err != nil {
			return nil, err
		}

		contactResponse := Contact{}

		if err := json.Unmarshal(jsonBlob, &contactResponse); err != nil {
			return nil, err
		}

		response[i] = contactResponse
	}

	return response, err
}

func SObjectDescribe(api *Api, response *SObjectDescribeResponse) error {
	jsonBlob, err := request(api, "GET", SObjectDescribePath, map[string]string{}, "")
	if err != nil {
		return err
	}

	if err := json.Unmarshal(jsonBlob, response); err != nil {
		return err
	}

	return nil
}

func Describe(api *Api, response []DescribeResponse) error {
	jsonBlob, err := request(api, "GET", DescribePath, map[string]string{}, "")
	if err != nil {
		return err
	}

	//It could be a single response...
	singleResponse := DescribeResponse{}
	if err := json.Unmarshal(jsonBlob, &singleResponse); err != nil {
		//Or multiple because the API hates you when it spits out errors...
		if err2 := json.Unmarshal(jsonBlob, &response); err != nil {
			return err2
		}
		return err
	} else {
		response[0] = singleResponse
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
