package admin

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"appengine/urlfetch"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	stripe "crowdstart.io/thirdparty/stripe/models"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

type stripeToken struct {
	AccessToken          string `json:"access_token"`
	Error                string `json:"error"`
	ErrorDescription     string `json:"error_description"`
	Livemode             bool   `json:"livemode"`
	RefreshToken         string `json:"refresh_token"`
	Scope                string `json:"scope"`
	StripePublishableKey string `json:"stripe_publishable_key"`
	StripeUserId         string `json:"stripe_user_id"`
	TokenType            string `json:"token_type"`
}

// StripeCallback Stripe End Points
func StripeCallback(c *gin.Context) {
	req := c.Request
	code := req.URL.Query().Get("code")
	errStr := req.URL.Query().Get("error")

	// Failed to get back authorization code from Stripe
	if errStr != "" {
		template.Render(c, "stripe/connect.html", "error", errStr)
		return
	}

	ctx := middleware.GetAppEngine(c)
	client := urlfetch.Client(ctx)

	data := url.Values{}
	data.Set("client_secret", config.Stripe.APISecret)
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

	token := new(stripeToken)

	// try and extract the json struct
	if err := json.Unmarshal(jsonBlob, token); err != nil {
		c.Fail(500, err)
	}

	// Stripe returned an error
	if token.Error != "" {
		template.Render(c, "connect.html",
			"stripeError", token.Error,
			"stripe", config.Stripe,
			"salesforce", config.Salesforce)
		return
	}

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

	// Update stripe data
	campaign.Stripe.AccessToken = token.AccessToken
	campaign.Stripe.Livemode = token.Livemode
	campaign.Stripe.PublishableKey = token.StripePublishableKey
	campaign.Stripe.RefreshToken = token.RefreshToken
	campaign.Stripe.Scope = token.Scope
	campaign.Stripe.TokenType = token.TokenType
	campaign.Stripe.UserId = token.StripeUserId

	// Update in datastore
	if _, err := db.PutKey("campaign", email, campaign); err != nil {
		log.Panic("Failed to update campaign: %v", err)
	}

	// Success
	template.Render(c, "stripe/success.html", "token", token.AccessToken)
}

type Event struct {
	ApiVersion      string  `json:"api_version"`
	Created         float64 `json:"created"`
	ID              string  `json:"id"`
	Livemode        bool    `json:"livemode"`
	Object          string  `json:"object"`
	PendingWebhooks float64 `json:"pending_webhooks"`
	Request         string  `json:"request"`
	Type            string  `json:"type"`
}

type ChargeEvent struct {
	Event
	Data struct {
		Charge models.Charge `json:"object"`
	} `json:"data"`
}

type DisputeEvent struct {
	Event
	Data struct {
		Dispute stripe.Dispute `json:"Object"`
	} `json:"data"'`
}

type AccountUpdatedEvent struct {
	Event
	Data struct {
		Account stripe.Account `json:"object"`
	} `json:"data"`
}

// StripeCallback Stripe End Points
func StripeWebhook(c *gin.Context) {
	data, err := ioutil.ReadAll(c.Request.Body)
	log.Debug("%#v", err)
	log.Debug("%#v", string(data[:]))

	if c.Request.Method == "POST" {
		event := new(Event)
		if err := json.Unmarshal(data, event); err != nil {
			c.String(500, "Error parsing event json")
			return
		}
		if !event.Livemode {
			c.String(200, event.Type)
			return
		}

		switch event.Type {
		case "charge.succeeded":
		case "charge.refunded":
		case "charge.failed":
		case "charge.captured":
		case "charge.updated":
			chargeModified(c, data)

		case "charge.dispute.created":
		case "charge.dispute.updated":
		case "charge.dispute.closed":
		case "charge.dispute.funds_withdrawn":
		case "charge.dispute.funds_reinstated":
			c.String(501, "Dispute events are temporarily disabled")
			// chargeDisputed(c, data)

		case "account.updated":
			accountUpdated(c, data)

		case "ping":
			c.String(200, "pong")
		}
	}
}

func chargeModified(c *gin.Context, data []byte) {
	chargeEvt := new(ChargeEvent)
	if err := json.Unmarshal(data, chargeEvt); err != nil {
		c.String(500, "Error parsing charge json")
		log.Panic(err)
	}
	charge := chargeEvt.Data.Charge

	db := datastore.New(c)
	order := new(models.Order)
	key, err := db.Query("order").Filter("Charges.ID =", charge.ID).Run(db.Context).Next(order)
	if err != nil {
		c.String(500, "Error retrieving order by charge id")
		log.Panic(err)
	}

	for i := range order.Charges {
		if order.Charges[i].ID == charge.ID {
			order.Charges[i] = charge
			break
		}
	}

	if _, err := db.PutKey("order", key, order); err != nil {
		c.String(500, "Error saving order")
		log.Panic(err)
	}

	c.String(200, "ok")
}

// func chargeDisputed(c *gin.Context, data []byte) {
// 	event := new(DisputeEvent)
// 	if err := json.Unmarshal(data, event); err != nil {
// 		c.String(500, "Error parsing dispute json")
// 		log.Panic(err)
// 	}
// 	dispute := event.Data.Dispute

// 	db := datastore.New(c)
// 	order := new(models.Order)
// 	key, err := db.Query("order").Filter("Charges.ID =", dispute.Charge).Run(db.Context).Next(order)
// 	if err != nil {
// 		c.String(500, "Error retrieving order")
// 		log.Panic(err)
// 	}

// 	for i, charge := range order.Charges {
// 		if charge.ID == dispute.Charge {
// 			order.Charges[i].Dispute = dispute
// 			break
// 		}
// 	}

// 	if _, err := db.PutKey("order", key, order); err != nil {
// 		c.String(500, "Error saving order")
// 		log.Panic(err)
// 	}

// 	c.String(200, "ok")
// }

func accountUpdated(c *gin.Context, data []byte) {
	event := new(AccountUpdatedEvent)
	if err := json.Unmarshal(data, event); err != nil {
		c.String(500, "Error parsing account json")
		log.Panic(err)
	}
	customerId := event.Data.Account.ID
	db := datastore.New(c)

	user := new(models.User)
	key, err := db.Query("user").
		Filter("Stripe.CustomerId =", customerId).
		Run(db.Context).
		Next(user)
	if err != nil {
		c.String(500, "Error retrieving user")
		log.Panic(err)
	}

	user.Stripe.Account = event.Data.Account

	_, err = db.PutKey("user", key, user)
	if err != nil {
		c.String(500, "Error saving user")
		log.Panic(err)
	}

	c.String(200, "ok")
}
