package stripe

import (
	"encoding/json"
	"io/ioutil"

	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models"
	stripe "crowdstart.io/thirdparty/stripe/models"
	"crowdstart.io/util/log"
)

func StripeSync(c *gin.Context) {
	c.String(200, "Synchronizing charges")
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
			chargeDisputed(c, data)

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

	if _, err := db.PutKind("order", key, order); err != nil {
		c.String(500, "Error saving order")
		log.Panic(err)
	}

	c.String(200, "ok")
}

func chargeDisputed(c *gin.Context, data []byte) {
	event := new(DisputeEvent)
	if err := json.Unmarshal(data, event); err != nil {
		c.String(500, "Error parsing dispute json")
		log.Panic(err)
	}
	dispute := event.Data.Dispute

	db := datastore.New(c)
	order := new(models.Order)
	key, err := db.Query("order").Filter("Charges.ID =", dispute.Charge).Run(db.Context).Next(order)
	if err != nil {
		c.String(500, "Error retrieving order")
		log.Panic(err)
	}

	for i, charge := range order.Charges {
		if charge.ID == dispute.Charge {
			order.Charges[i].Disputed = false
			break
		}
	}

	if _, err := db.Put("dispute", &dispute); err != nil {
		c.String(500, "Error saving dispute")
		log.Panic(err)
	}

	if _, err := db.PutKind("order", key, order); err != nil {
		c.String(500, "Error saving order")
		log.Panic(err)
	}

	c.String(200, "ok")
}

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

	_, err = db.PutKind("user", key, user)
	if err != nil {
		c.String(500, "Error saving user")
		log.Panic(err)
	}

	c.String(200, "ok")
}
