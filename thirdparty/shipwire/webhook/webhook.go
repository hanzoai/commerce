package webhook

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/order"
	"hanzo.io/thirdparty/shipwire/response"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/util/log"

	. "hanzo.io/models"
)

// Webhook Response Bodies
type MessageBody struct {
	Status           string          `json:"status"`
	Message          string          `json:"message"`
	Resource         json.RawMessage `json:"resource"`
	ResourceLocation string          `json:"resourceLocation"`
}

// Webhook Responses
type Message struct {
	Topic                 string      `json:"topic"`
	Attempt               string      `json:"attempt"`
	Timestamp             time.Time   `json:"timestamp"`
	UniqueEventId         string      `json:"uniqueEventID"`
	WebhookSubscriptionId int         `json:"webhookSubscriptionID"`
	Body                  MessageBody `json:"body"`
}

func Send200(c *gin.Context) {
	c.String(200, "ok\n")
}

func Process(c *gin.Context) {
	var req Message
	if err := json.Decode(c.Request.Body, req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	switch req.Topic {
	case "tracking.created", "tracking.updated", "tracking.delivered":
		var r response.Tracking
		if err := json.Unmarshal(req.Body.Resource, &r); err != nil {
			msg := fmt.Sprintf("Failed decode resource: %v\n%v", err, req.Body.Resource)
			http.Fail(c, 400, msg, err)
		}

		tracking(c, r)
	default:
		c.String(200, "ok\n")
	}
}

func tracking(c *gin.Context, t response.Tracking) {
	log.Warn("Tracking Information:\n%v", t, c)

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	ord := order.New(db)
	id := t.OrderExternalId //[1:]
	err := ord.GetById(id)
	if err != nil {
		log.Warn("Unable to find order '%s': %v", id, err, c)
		c.String(200, "ok\n")
		return
	}

	if err := ord.GetById(t.OrderExternalId); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	ord.Fulfillment.TrackingNumber = t.Tracking
	if !t.LabelCreatedDate.IsZero() {
		ord.FulfillmentStatus = FulfillmentLabelled
	} else if !t.TrackedDate.IsZero() {
		ord.FulfillmentStatus = FulfillmentProcessing
	} else if !t.FirstScanDate.IsZero() {
		ord.FulfillmentStatus = FulfillmentShipped
	} else if !t.DeliveredDate.IsZero() {
		ord.FulfillmentStatus = FulfillmentDelivered
	}
	ord.Fulfillment.CreatedAt = t.LabelCreatedDate
	ord.Fulfillment.ShippedAt = t.FirstScanDate
	ord.Fulfillment.DeliveredAt = t.DeliveredDate
	// ord.Fulfillment.Service = req.Service
	ord.Fulfillment.Carrier = t.Carrier
	ord.Fulfillment.Carrier = t.Summary

	// usr := user.New(db)
	// usr.MustGetById(ord.UserId)

	// pay := payment.New(db)
	// pay.MustGetById(ord.PaymentIds[0])

	// emails.SendFulfillmentEmail(db.Context, org, ord, usr, pay)
	ord.MustPut()

	// emails.SendFulfillmentEmail(db.Context, org, ord, usr, pay)

	c.String(200, "ok\n")
}
