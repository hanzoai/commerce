package webhook

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/organization"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/thirdparty/stripe/tasks"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
	"crowdstart.com/util/router"
)

// Handle stripe webhook POSTs
func Webhook(c *gin.Context) {
	event := new(stripe.Event)
	if err := json.Decode(c.Request.Body, event); err != nil {
		c.String(500, "Error parsing event json")
		return
	}

	// log.Debug("Received '%s' event: %+v", event.Type, event, c)

	// Look up organization
	db := datastore.New(c)
	org := organization.New(db)
	ok, err := org.Query().Filter("Stripe.UserId=", event.UserID).First()
	if err != nil {
		log.Error("Failed to query organization using Stripe UserId '%s': %v", event.UserID, err, c)
		return
	}

	if !ok {
		log.Warn("No organization found with Stripe UserId '%s': %#v", event.UserID, event, c)
		return
	}

	// Get stripe token
	token, err := org.GetStripeAccessToken(event.UserID)
	if err != nil {
		log.Error("Failed to get Stripe access token for organization '%s': %v", org.Name, err, c)
		return
	}

	ctx := middleware.GetAppEngine(c)

	switch event.Type {
	case "charge.succeeded":
		//Do Nothing
	case "charge.captured", "charge.failed", "charge.refunded", "charge.updated":
		ch := stripe.Charge{}
		if err := json.Unmarshal(event.Data.Raw, &ch); err != nil {
			log.Error("Failed to unmarshal stripe.Charge %#v: %v", event, err, c)
		} else {
			start := time.Now()
			tasks.ChargeSync.Call(ctx, org.Name, token, ch, start)
		}
	case "charge.dispute.closed", "charge.dispute.created", "charge.dispute.funds_reinstated", "charge.dispute.funds_withdrawn", "charge.dispute.updated":
		dispute := stripe.Dispute{}
		if err := json.Unmarshal(event.Data.Raw, &dispute); err != nil {
			log.Error("Failed to unmarshal stripe.Dispute %#v: %v", event, err, c)
		} else {
			start := time.Now()
			tasks.DisputeSync.Call(ctx, org.Name, token, dispute, start)
		}
	case "transfer.created", "transfer.failed", "transfer.paid", "transfer.reversed", "transfer.updated":
		transfer := stripe.Transfer{}
		if err := json.Unmarshal(event.Data.Raw, &transfer); err != nil {
			log.Error("Failed to unmarshal stripe.Transfer %#v: %v", event, err, c)
		} else {
			start := time.Now()
			tasks.TransferSync.Call(ctx, org.Name, token, transfer, start)
		}
	case "ping":
		c.String(200, "pong")
		return

	default:
		log.Warn("Unsupported Stripe event '%s': %#v", event.Type, event, c)
		return
	}

	c.String(200, "ok")
}

// Wire up webhook endpoint
func Route(router router.Router, args ...gin.HandlerFunc) {
	api := router.Group("stripe")
	api.POST("/webhook", Webhook)
}
