package webhook

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/thirdparty/stripe/tasks"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
)

// Handle stripe webhook POSTs
func Webhook(c *gin.Context) {
	event := new(stripe.Event)
	if err := json.Decode(c.Request.Body, event); err != nil {
		c.String(500, "Error parsing event json")
		return
	}
	if !event.Live {
		c.String(200, event.Type)
		return
	}

	switch event.Type {
	case "charge.captured":
	case "charge.failed":
	case "charge.refunded":
	case "charge.succeeded":
	case "charge.updated":
		if event.Type == "charge.updated" {
			log.Debug("WTF IS A CHARGE UPDATED: %#v", event)
		}

		// Decode stripe charge
		ch := stripe.Charge{}
		json.Unmarshal(event.Data.Raw, &ch)

		ctx := middleware.GetAppEngine(c)

		start := time.Now()
		tasks.UpdatePayment.Call(ctx, ch, start)

	case "charge.dispute.closed":
	case "charge.dispute.created":
	case "charge.dispute.funds_reinstated":
	case "charge.dispute.funds_withdrawn":
	case "charge.dispute.updated":
		dispute := stripe.Dispute{}
		if err := json.Unmarshal(event.Data.Raw, &dispute); err != nil {
			log.Error("Error decoding dispute. %#v %#v", event, err, c)
			return
		}
		start := time.Now()
		tasks.UpdateDisputedPayment.Call(middleware.GetAppEngine(c), dispute, start)

	case "ping":
		c.String(200, "pong")
	default:
		log.Warn("Unknown Stripe webhook event %s %#v", event.Type, event, c)
	}
}
