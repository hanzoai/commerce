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

	log.Debug("Received event %#v", *event, c)

	if !event.Live {
		c.String(200, event.Type)
		return
	}

	db := datastore.New(c)
	org := organization.New(db)
	if _, err := org.Query().Filter("Stripe.UserId=", event.UserID).First(); err != nil {
		log.Error("Failed to look up organization by Stripe user id: %v", err, c)
		return
	}
	token, err := org.GetStripeAccessToken(event.UserID)
	if err != nil {
		log.Error("Failed to get Stripe access token from organization: %#v %v", org, err, c)
		return
	}

	ctx := middleware.GetAppEngine(c)

	switch event.Type {
	case "charge.captured":
	case "charge.failed":
	case "charge.refunded":
	case "charge.succeeded":
	case "charge.updated":
		// Decode stripe charge
		ch := stripe.Charge{}
		if err := json.Unmarshal(event.Data.Raw, &ch); err != nil {
			log.Error("Failed to unmarshal stripe.Charge: %#v %v", event, err, c)
			return
		}

		start := time.Now()
		tasks.ChargeSync.Call(ctx, org.Name, token, ch, start)

	case "charge.dispute.closed":
	case "charge.dispute.created":
	case "charge.dispute.funds_reinstated":
	case "charge.dispute.funds_withdrawn":
	case "charge.dispute.updated":
		// Decode stripe dispute
		dispute := stripe.Dispute{}
		if err := json.Unmarshal(event.Data.Raw, &dispute); err != nil {
			log.Error("Failed to unmarshal stripe.Dispute: %#v %v", event, err, c)
			return
		}

		start := time.Now()
		tasks.DisputeSync.Call(ctx, org.Name, token, dispute, start)

	case "ping":
		c.String(200, "pong")
	default:
		log.Info("Unsupported Stripe webhook event %s %#v", event.Type, event, c)
	}
}

// Wire up webhook endpoint
func Route(router router.Router, args ...gin.HandlerFunc) {
	api := router.Group("stripe")

	api.POST("/webhook", Webhook)
}
