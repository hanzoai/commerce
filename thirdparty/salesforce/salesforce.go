package salesforce

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"appengine"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/log"

	"appengine/urlfetch"
)

type Api struct {
	lastRequest  *http.Request
	lastResponse *http.Response

	LastStatusCode int
	LastBody       []byte
	Context        appengine.Context

	Campaign *models.Campaign

	Update bool
}

// Get the HttpClient from the Gin context
func getClient(c appengine.Context) *http.Client {
	client := urlfetch.Client(c)

	return client
}

// Request sends HTTP requests to Salesforce
func (a *Api) request(method, path, data string, headers *map[string]string, retry bool) error {
	c := a.Context
	client := getClient(c)
	url := a.Campaign.Salesforce.InstanceUrl + path

	log.Debug("Creating a Request to %v to %v", method, url, c)
	req, err := http.NewRequest(method, url, strings.NewReader(data))
	if err != nil {
		log.Error("Could not create Request: %v", err)
		return err
	}

	req.Header.Set("Authorization", "Bearer "+a.Campaign.Salesforce.AccessToken)
	if headers != nil {
		for key, value := range *headers {
			req.Header.Set(key, value)
		}
		log.Debug("Setting Headers %v", req.Header, c)
	}

	a.lastRequest = req

	log.Debug("Sending Request", c)

	res, err := client.Do(req)
	if err != nil {
		log.Error("Request has failed: %v", err, c)
		return err
	}
	defer res.Body.Close()

	log.Debug("Decoding Response", c)

	a.lastResponse = res

	log.Debug("Decoding Body", c)
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Error("Could not read Response Body: %v", err, c)
		return err
	}

	a.LastStatusCode = res.StatusCode
	a.LastBody = body

	if len(body) == 0 {
		log.Warn("The Response has no Body", c)
		return nil
	}

	responses := make([]SalesforceError, 1)

	log.Debug("Try Decoding any Errors in the Response", c)
	if err = json.Unmarshal(body, &responses); err != nil {
		log.Debug("No Errors in the Response", c)
		return nil
	}

	if retry {
		log.Debug("Encountered Error '%v: %v'", responses[0].ErrorCode, responses[0].Message, c)
		if responses[0].ErrorCode == "INVALID_SESSION_ID" {
			log.Debug("Refreshing Token", c)
			if err := a.Refresh(); err != nil {
				return errors.New(fmt.Sprintf("%v: %v", responses[0].ErrorCode, responses[0].Message))
			}
			return a.request(method, path, data, headers, false)
		}
	}
	return errors.New(fmt.Sprintf("%v, %v", string(a.LastBody[:]), err, c))
}

// New creates an API from a Context and Campaign
func New(c appengine.Context, campaign *models.Campaign, update bool) *Api {
	return &Api{
		Campaign: campaign,
		Context:  c,
		Update:   update,
	}
}

// Refresh refreshes the Salesforce tokens and saves them to database
func (a *Api) Refresh() error {
	c := a.Context
	client := urlfetch.Client(c)

	// https://help.salesforce.com/HTViewHelpDoc?id=remoteaccess_oauth_refresh_token_flow.htm&language=en_US
	data := url.Values{}
	data.Add("grant_type", "refresh_token")
	data.Set("client_id", config.Salesforce.ConsumerKey)
	data.Set("client_secret", config.Salesforce.ConsumerSecret)
	data.Set("refresh_token", a.Campaign.Salesforce.RefreshToken)

	log.Debug("Posting to the Refresh API", c)
	tokenReq, err := http.NewRequest("POST", LoginUrl, strings.NewReader(data.Encode()))
	if err != nil {
		log.Error("Could not create request: %v", err, c)
		return err
	}

	// try to post to refresh token API
	res, err := client.Do(tokenReq)
	if err != nil {
		log.Error("Request Failed: %v", err, c)
		return err
	}
	defer res.Body.Close()

	// decode the json
	jsonBlob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Error("Could not decode Body: %v", err, c)
		return err
	}

	response := new(SalesforceTokenResponse)

	log.Debug("Trying to unmarshal Body: %s", jsonBlob)
	// try and extract the json struct
	if err = json.Unmarshal(jsonBlob, response); err != nil {
		log.Error("Could not unmarshal: %v", err, c)
		return err
	}

	if response.Error != "" {
		log.Error("%v: %v", response.Error, response.ErrorDescription, c)
		return errors.New(fmt.Sprintf("%v: %v", response.Error, response.ErrorDescription))
	}

	log.Debug("New Access Token: %v", response.AccessToken, c)
	a.Campaign.Salesforce.AccessToken = response.AccessToken
	a.Campaign.Salesforce.InstanceUrl = response.InstanceUrl
	a.Campaign.Salesforce.Id = response.Id
	a.Campaign.Salesforce.IssuedAt = response.IssuedAt
	a.Campaign.Salesforce.Signature = response.Signature

	log.Debug("Updating Campaign", c)
	if a.Update {
		db := datastore.New(c)
		db.PutKey("campaign", a.Campaign.Id, a.Campaign)
	}

	return nil
}

func (a *Api) Push(object interface{}) error {
	c := a.Context

	if object == nil {
		return errors.New("Cannot Push nil object")
	}

	switch v := object.(type) {
	case *models.User:
		if v.Id == "" {
			return errors.New("Id is required for Upsert")
		}
		account := Account{}
		account.FromUser(v)
		accountBytes, err := json.Marshal(&account)
		if err != nil {
			return err
		}

		accountJSON := string(accountBytes[:])
		path := fmt.Sprintf(AccountExternalIdPath, strings.Replace(v.Id, ".", "_", -1))

		log.Debug("Upserting Account: %v", account, c)
		if err = a.request("PATCH", path, accountJSON, &map[string]string{"Content-Type": "application/json"}, true); err != nil {
			return err
		}

		contact := Contact{}
		contact.FromUser(v)

		contactBytes, err := json.Marshal(&contact)
		if err != nil {
			return err
		}

		contactJSON := string(contactBytes[:])
		path = fmt.Sprintf(ContactExternalIdPath, strings.Replace(v.Id, ".", "_", -1))

		log.Debug("Upserting Contact: %v", contact, c)
		if err = a.request("PATCH", path, contactJSON, &map[string]string{"Content-Type": "application/json"}, true); err != nil {
			return err
		}

	case *models.Order:
		log.Debug("Upserting Order", c)
		if v.Id == "" {
			return errors.New("Id is required for Upsert")
		}

		order := Order{}
		order.FromOrder(v)

		log.Debug("Converting to Order: %v", order, c)
		orderBytes, err := json.Marshal(&order)
		if err != nil {
			return err
		}

		orderJSON := string(orderBytes[:])
		path := fmt.Sprintf(OrderExternalIdPath, strings.Replace(v.Id, ".", "_", -1))

		log.Debug("Upserting Order: %v", order, c)
		if err = a.request("PATCH", path, orderJSON, &map[string]string{"Content-Type": "application/json"}, true); err != nil {
			return err
		}

	default:
		return errors.New("Invalid Type")
	}

	if len(a.LastBody) == 0 {
		if a.LastStatusCode == 201 || a.LastStatusCode == 204 {
			log.Debug("Upsert returned %v", a.LastStatusCode, c)
			return nil
		} else {
			return errors.New(fmt.Sprintf("Request returned unexpected status code %v", a.LastStatusCode))
		}
	}

	response := new(UpsertResponse)

	if err := json.Unmarshal(a.LastBody, response); err != nil {
		log.Error("Could not unmarshal: %v", string(a.LastBody[:]), c)
		return err
	}

	if !response.Success {
		log.Error("Upsert Failed: %v: %v", response.Errors[0].ErrorCode, response.Errors[0].Message, c)
		return errors.New(fmt.Sprintf("%v: %v", response.Errors[0].ErrorCode, response.Errors[0].Message))
	}

	return nil
}

func (a *Api) Pull(id string, object interface{}) error {
	c := a.Context

	if object == nil {
		return errors.New("Cannot Pull nil object")
	}

	switch v := object.(type) {
	case *models.User:
		log.Debug("Getting User", c)
		if id == "" {
			return errors.New("Id is required for Get")
		}

		path := fmt.Sprintf(ContactExternalIdPath, id)

		if err := a.request("GET", path, "", nil, true); err != nil {
			return err
		}

		contact := new(Contact)

		if err := json.Unmarshal(a.LastBody, contact); err != nil {
			log.Error("Could not unmarshal: %v", string(a.LastBody[:]), c)
			return err
		}

		path = fmt.Sprintf(AccountExternalIdPath, id)

		if err := a.request("GET", path, "", nil, true); err != nil {
			return err
		}

		account := new(Account)

		if err := json.Unmarshal(a.LastBody, account); err != nil {
			log.Error("Could not unmarshal: %v", string(a.LastBody[:]), c)
			return err
		}

		log.Debug("Getting Contact: %v", contact, c)

		log.Debug("Converting to User", c)
		contact.ToUser(v)
	default:
		return errors.New("Invalid Type")
	}

	return nil
}

func (a *Api) PullUpdated(start, end time.Time, objects interface{}) error {
	c := a.Context

	switch v := objects.(type) {
	case *[]*models.User:
		log.Debug("Getting Updated Contacts", c)
		path := fmt.Sprintf(ContactsUpdatedPath, start.Format(time.RFC3339), end.Format(time.RFC3339))

		if err := a.request("GET", path, "", nil, true); err != nil {
			return err
		}

		response := new(UpdatedRecordsResponse)

		if err := json.Unmarshal(a.LastBody, &response); err != nil {
			return err
		}

		var user *models.User
		var ok bool

		users := make(map[string]*models.User)

		for _, id := range response.Ids {
			log.Debug("Getting Contact for ")
			path := fmt.Sprintf(ContactPath, id)
			if err := a.request("GET", path, "", nil, true); err != nil {
				log.Warn("Failed to Get Contact for %v", id, c)
				continue
			}

			contact := new(Contact)
			if err := json.Unmarshal(a.LastBody, contact); err != nil {
				log.Warn("Could not unmarshal: %v", string(a.LastBody[:]), c)
				continue
			}

			// We key based on accountId because it is common to both contacts and accounts
			if user, ok = users[contact.AccountId]; !ok {
				user = new(models.User)
				users[contact.AccountId] = user
			}

			log.Debug("Getting Contact: %v %v", contact, user, c)
			contact.ToUser(user)
		}

		log.Debug("Getting Updated Accounts", c)
		path = fmt.Sprintf(AccountsUpdatedPath, start.Format(time.RFC3339), end.Format(time.RFC3339))

		if err := a.request("GET", path, "", nil, true); err != nil {
			return err
		}

		response = new(UpdatedRecordsResponse)

		if err := json.Unmarshal(a.LastBody, &response); err != nil {
			return err
		}

		for _, id := range response.Ids {
			if user, ok = users[id]; !ok {
				user = new(models.User)
				users[id] = user
			}

			path = fmt.Sprintf(AccountPath, id)
			if err := a.request("GET", path, "", nil, true); err != nil {
				log.Warn("Failed to Get Account for %v", id, c)
				continue
			}

			account := new(Account)
			if err := json.Unmarshal(a.LastBody, account); err != nil {
				log.Warn("Could not unmarshal: %v", string(a.LastBody[:]), c)
				continue
			}

			log.Debug("Getting Account: %v", account, c)

			account.ToUser(user)
		}

		log.Debug("Pulled %v Users %v", len(users), c)
		userSlice := make([]*models.User, len(users))

		i := 0
		for _, u := range users {
			userSlice[i] = u
			i++
		}

		*v = userSlice
	default:
		return errors.New("Invalid Type")
	}

	return nil
}

func (a *Api) SObjectDescribe(api *Api, response *SObjectDescribeResponse) error {
	c := a.Context

	if err := api.request("GET", SObjectDescribePath, "", nil, true); err != nil {
		return err
	}

	if err := json.Unmarshal(a.LastBody, response); err != nil {
		log.Error("Could not unmarshal: %v", string(a.LastBody[:]), c)
		return err
	}

	return nil
}

func (a *Api) Describe(response *DescribeResponse) error {
	c := a.Context

	if err := a.request("GET", DescribePath, "", nil, true); err != nil {
		return err
	}

	//It could be a single response...
	if err := json.Unmarshal(a.LastBody, response); err != nil {
		//Or multiple because the API hates you when it spits out errors...
		var errResponse *[]SalesforceError
		if err2 := json.Unmarshal(a.LastBody, errResponse); err2 != nil {
			log.Error("Could not unmarshal: %v", string(a.LastBody[:]), c)
			return err2
		} else {
			log.Error("%v: %v", (*errResponse)[0].ErrorCode, (*errResponse)[0].Message, c)
		}
		return err
	}

	return nil
}
