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

type RefundEvent struct {
	Event
	Data struct {
		Object stripe.Charge `json:"object"`
	} `json:"data"`
}

// StripeCallback Stripe End Points
func StripeWebhook(c *gin.Context) {
	data, err := ioutil.ReadAll(c.Request.Body)
	log.Info("%#v", err)
	log.Info("%#v", string(data[:]))

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
		case "charge.refunded":
			refund(c, data)
		}
	}
}

func refund(c *gin.Context, data []byte) {
	refundEvt := new(RefundEvent)

	err := json.Unmarshal(data, refundEvt)
	if err != nil {
		log.Debug(err)
		log.Debug(len(data))
		c.String(500, "Error parsing refund json")
		return
	}

	// if !c.Bind(refundEvt) {
	// 	c.String(500, "Error parsing refund json")
	// 	return
	// }
	if refundEvt.Data.Object.Refunded {
		for _, charge := range refundEvt.Data.Object.Refunds.Data {
			db := datastore.New(c)
			chargeId := charge.ID
			var orders []models.Order
			keys, err := db.Query("order").
				Filter("Charges.ID =", chargeId).
				Limit(1).
				GetAll(db.Context, &orders)
			if err != nil {
				c.String(500, "Unable to find order")
				continue
			}
			if len(orders) < 1 {
				c.String(500, "Unable to find order")
				continue
			}

			order := orders[0]
			for i := range order.Charges {
				if order.Charges[i].ID == chargeId {
					order.Charges[i].Refunded = true
					break
				}
			}

			order.Refunded = true // TODO verify if this is the required behaviour

			if _, err := db.PutKey("order", keys[0], order); err != nil {
				c.String(500, "Error saving order")
				continue
			}
			c.String(200, "ok")
		}
	}
}
