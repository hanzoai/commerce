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
		Object struct {
			Amount             float64 `json:"amount"`
			AmountRefunded     float64 `json:"amount_refunded"`
			BalanceTransaction string  `json:"balance_transaction"`
			Captured           bool    `json:"captured"`
			Card               struct {
				AddressCity       string  `json:"address_city"`
				AddressCountry    string  `json:"address_country"`
				AddressLine1      string  `json:"address_line1"`
				AddressLine1Check string  `json:"address_line1_check"`
				AddressLine2      string  `json:"address_line2"`
				AddressState      string  `json:"address_state"`
				AddressZip        string  `json:"address_zip"`
				AddressZipCheck   string  `json:"address_zip_check"`
				Brand             string  `json:"brand"`
				Country           string  `json:"country"`
				Customer          string  `json:"customer"`
				CvcCheck          string  `json:"cvc_check"`
				DynamicLast4      string  `json:"dynamic_last4"`
				ExpMonth          float64 `json:"exp_month"`
				ExpYear           float64 `json:"exp_year"`
				Fingerprint       string  `json:"fingerprint"`
				Funding           string  `json:"funding"`
				ID                string  `json:"id"`
				Last4             string  `json:"last4"`
				Name              string  `json:"name"`
				Object            string  `json:"object"`
			} `json:"card"`
			Created        float64           `json:"created"`
			Currency       string            `json:"currency"`
			Customer       string            `json:"customer"`
			Description    string            `json:"description"`
			Dispute        string            `json:"dispute"`
			FailureCode    string            `json:"failure_code"`
			FailureMessage string            `json:"failure_message"`
			Fee            float64           `json:"fee"`
			FraudDetails   map[string]string `json:"fraud_details"`
			ID             string            `json:"id"`
			Invoice        string            `json:"invoice"`
			Livemode       bool              `json:"livemode"`
			Metadata       map[string]string `json:"metadata"`
			Object         string            `json:"object"`
			Paid           bool              `json:"paid"`
			ReceiptEmail   string            `json:"receipt_email"`
			ReceiptNumber  string            `json:"receipt_number"`
			Refunded       bool              `json:"refunded"`
			Refunds        struct {
				Data []struct {
					Amount             float64           `json:"amount"`
					BalanceTransaction string            `json:"balance_transaction"`
					Charge             string            `json:"charge"`
					Created            float64           `json:"created"`
					Currency           string            `json:"currency"`
					ID                 string            `json:"id"`
					Metadata           map[string]string `json:"metadata"`
					Object             string            `json:"object"`
					Reason             string            `json:"reason"`
					ReceiptNumber      string            `json:"receipt_number"`
				} `json:"data"`
				HasMore    bool    `json:"has_more"`
				Object     string  `json:"object"`
				TotalCount float64 `json:"total_count"`
				URL        string  `json:"url"`
			} `json:"refunds"`
			Shipping             string `json:"shipping"`
			StatementDescription string `json:"statement_description"`
			StatementDescriptor  string `json:"statement_descriptor"`
		} `json:"object"`
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
