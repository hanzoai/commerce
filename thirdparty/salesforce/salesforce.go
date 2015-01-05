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

//Paths
var LoginUrl = "https://login.salesforce.com/services/oauth2/token"
var DescribePath = "/services/data/v29.0/"
var SObjectDescribePath = DescribePath + "sobjects/"
var ContactQueryPath = DescribePath + "query/?q=SELECT+Id+from+Contact+where+Contact.Email+=+%27%v%27"
var ContactUpsertUsingEmailPath = SObjectDescribePath + "Contact/Email/%v"

// Salesforce Structs
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

type SObjectUrls struct {
	SObject     string `json:"sobject"`
	Describe    string `json:"describe"`
	RowTemplate string `json:"rowTemplate"`
}

type SObject struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	KeyPrefix   string `json:"keyPrefix"`
	LabelPlural string `json:"labelPlural"`

	Urls SObjectUrls `json:"urls"`

	// grammatically annoying bools
	Custom              bool `json:"custom"`
	Layoutable          bool `json:"layoutable"`
	Activateable        bool `json:"activateable"`
	Searchable          bool `json:"searchable"`
	Updateable          bool `json:"updateable"`
	Createable          bool `json:"createable"`
	DeprecatedAndHidden bool `json:"deprecatedAndHidden"`
	CustomSetting       bool `json:"customSetting"`
	Deletable           bool `json:"deletable"`
	FeedEnable          bool `json:"feedEnabled"`
	Mergeable           bool `json:"mergeable"`
	Queryable           bool `json:"queryable"`
	Replicateable       bool `json:"replicateable"`
	Retrieveable        bool `json:"retrieveable"`
	Undeleteable        bool `json:"undeleteable"`
	Triggerable         bool `json:"triggerable"`
}

type SObjectDescribeResponse struct {
	Encoding     string    `json:"encoding"`
	MaxBatchSize string    `json:"maxBatchSize"`
	SObjects     []SObject `json:"sobjects"`
}

type Attribute struct {
	Type string `json:"type"`
	Url  string `json:"url"`
}

type ContactQueryAttributes struct {
	Id         string    `json:"Id"`
	Attributes Attribute `json:"attributes"`
}

type ContactQueryResponse struct {
	TotalSize int                      `json:"totalSize"`
	Done      bool                     `json:"done"`
	Records   []ContactQueryAttributes `json:"attributes"`
}

type Contact struct {
	// Response Only Fields
	Attributes     Attribute `json:"attributes"`
	Id             string    `json:"Id"`
	IsDeleted      bool      `json:"IsDeleted"`
	MasterRecordId string    `json:"MasterRecordId"`
	AccountId      string    `json:"AccountId"`

	// Data Fields
	LastName                     string `json:"LastName"`
	FirstName                    string `json:"FirstName"`
	Salutation                   string `json:"Salutation"`
	Name                         string `json:"Name"`
	MailingStreet                string `json:"MailingStreet"`
	MailingCity                  string `json:"MailingCity"`
	MailingState                 string `json:"MailingState"`
	MailingPostalCode            string `json:"MailingPostalCode"`
	MailingCountry               string `json:"MailingCountry"`
	MailingStateCode             string `json:"MailingStateCode"`
	MailingCountryCode           string `json:"MailingCountryCode"`
	MailingLatitude              string `json:"MailingLatitude"`
	MailingLongitude             string `json:"MailingLongitude"`
	Phone                        string `json:"Phone"`
	Fax                          string `json:"Fax"`
	MobilePhone                  string `json:"MobilePhone"`
	ReportsToId                  string `json:"ReportsToId"`
	Email                        string `json:"tremallo@yahoo.com"`
	Title                        string `json:"Title"`
	Department                   string `json:"Department"`
	OwnerId                      string `json:"OwnerId"`
	CreatedDate                  string `json:"CreatedDate"`
	CreatedById                  string `json:"CreatedById"`
	LastModifiedDate             string `json:"LastModifiedDate"`
	LastModifiedById             string `json:"LastModifiedById"`
	SystemModstamp               string `json:"SystemModstamp"`
	LastActivityDate             string `json:"LastActivityDate"`
	LastCURequestDate            string `json:"LastCURequestDate"`
	LastCUUpdateDate             string `json:"LastCUUpdateDate"`
	LastViewedDate               string `json:"LastViewedDate"`
	LastReferencedDate           string `json:"LastReferencedDate"`
	EmailBouncedReason           string `json:"EmailBouncedReason"`
	EmailBouncedDate             string `json:"EmailBouncedDate"`
	IsEmailBounced               bool   `json:"IsEmailBounced"`
	JigsawContactId              string `json:"JigsawContactId"`
	ZendeskLastSyncDateC         string `json:"Zendesk__Last_Sync_Date__c"`
	ZendeskLastSyncStatusC       string `json:"Zendesk__Last_Sync_Status__c"`
	ZendeskResultC               string `json:"Zendesk__Result__c"`
	ZendeskTagsC                 string `json:"Zendesk__Tags__c"`
	ZendeskZendeskOutofSyncC     string `json:"Zendesk__Zendesk_OutofSync__c"`
	ZendeskZendeskOldTagsC       string `json:"Zendesk__Zendesk_oldTags__c"`
	ZendeskIsCreatedUpdatedFlagC string `json:"Zendesk__isCreatedUpdatedFlag__c"`
	ZendeskNotesC                string `json:"Zendesk__notes__c"`
	ZendeskZendeskIdC            string `json:"Zendesk__zendesk_id__c"`
	UniquePreorderLinC           string `json:"Unique_Preorder_Link__c"`
	FullfillmentStatusC          string `json:"Fulfillment_Status__c"`
	PreorderC                    string `json:"Preorder__c"`
	ShippingAddressC             string `json:"Shipping_Address__c"`
	ShippingCityC                string `json:"Shipping_City__c"`
	ShippingStateC               string `json:"Shipping_State__c"`
	ShippingPostalZipC           string `json:"Shipping_Postal_Zip__c"`
	ShippingCountryC             string `json:"Shipping_Country__c"`
	MC4SFMCSubscriberC           string `json:"MC4SF__MC_Subscriber__c"`
}

// Api Data Container
type Api struct {
	Tokens       SalesforceTokens
	LastJsonBlob string
}

func (a *Api) request(method, url string, data string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, strings.NewReader(data))
	if err != nil {
		log.Error("Could not create request: %v", err)
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.Tokens.AccessToken)

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
			Signature:    signature}})

	// Hit the topmost RESTful endpoint to test if credentials work
	response := make([]DescribeResponse, 1, 1)

	if err := Describe(api, c, response); err != nil {
		return nil, err
	}

	// If the endpoint has and error, try again after refreshing credentials
	if len(response) == 0 || response[0].ErrorCode != "" {
		// Try to get new API tokens by using the refresh token
		if err := Refresh(c, refreshToken, &api.Tokens); err != nil {
			return nil, err
		}

		// Try to hit the endpoint again
		err := Describe(api, c, response)
		if err != nil {
			return nil, err
		}

		if len(response) == 0 || response[0].ErrorCode != "" {
			return nil, errors.New("Nothing to decode")
		}
	}

	return api, nil
}

func request(api *Api, c *gin.Context, method, path string, headers map[string]string, data string) ([]byte, error) {
	client := getClient(c)

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

func UpsertContactByEmail(api *Api, c *gin.Context, contact *Contact) error {
	if contact.Email == "" {
		errors.New("Email is required")
	}

	contactBytes, err := json.Marshal(contact)
	if err != nil {
		return err
	}

	contactJSON := string(contactBytes[:])

	path := fmt.Sprintf(ContactUpsertUsingEmailPath, url.QueryEscape(contact.Email))

	jsonBlob, err := request(api, c, "POST", path, map[string]string{"Content-Type": "application/json"}, contactJSON)
	if err != nil {
		return err
	}

	api.LastJsonBlob = string(jsonBlob[:])

	return nil
}

func GetContactByEmail(api *Api, c *gin.Context, email string) ([]Contact, error) {
	path := fmt.Sprintf(ContactQueryPath, url.QueryEscape(email))

	jsonBlob, err := request(api, c, "GET", path, map[string]string{}, "")
	if err != nil {
		return nil, err
	}

	contactQueryResponse := new(ContactQueryResponse)

	if err := json.Unmarshal(jsonBlob, contactQueryResponse); err != nil {
		return nil, err
	}

	length := len(contactQueryResponse.Records)
	if length == 0 {
		return nil, errors.New("No records found")
	}

	response := make([]Contact, length, length)
	for i, record := range contactQueryResponse.Records {
		jsonBlob, err = request(api, c, "GET", record.Attributes.Url, map[string]string{}, "")
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

func SObjectDescribe(api *Api, c *gin.Context, response *SObjectDescribeResponse) error {
	jsonBlob, err := request(api, c, "GET", SObjectDescribePath, map[string]string{}, "")
	if err != nil {
		return err
	}

	if err := json.Unmarshal(jsonBlob, response); err != nil {
		return err
	}

	return nil
}

func Describe(api *Api, c *gin.Context, response []DescribeResponse) error {
	jsonBlob, err := request(api, c, "GET", DescribePath, map[string]string{}, "")
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
	}

	response[0] = singleResponse

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
