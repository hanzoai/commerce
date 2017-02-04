package webhook

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/order"
	"crowdstart.com/thirdparty/shipwire/response"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"

	. "crowdstart.com/models"
)

// Webhook Response Bodies
type TrackingMessageBody struct {
	response.TrackingRef

	Status  string `json:"status"`
	Message string `json:"message"`
}

// Webhook Responses
type Message struct {
	Topic                 string    `json:"topic"`
	Attempt               string    `json:"attempt"`
	Timestamp             time.Time `json:"timestamp"`
	UniqueEventId         string    `json:"uniqueEventID"`
	WebhookSubscriptionId int       `json:"webhookSubscriptionID"`
}

type TrackingMessage struct {
	Topic                 string              `json:"topic"`
	Attempt               string              `json:"attempt"`
	Timestamp             time.Time           `json:"timestamp"`
	UniqueEventId         string              `json:"uniqueEventID"`
	WebhookSubscriptionId int                 `json:"webhookSubscriptionID"`
	Body                  TrackingMessageBody `json:"body"`
}

func Send200(c *gin.Context) {
	c.String(200, "ok\n")
}

func Process(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	var trackingMsg *TrackingMessage

	if err := json.Decode(c.Request.Body, trackingMsg); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	switch trackingMsg.Topic {
	case "tracking.created", "tracking.updated", "tracking.delivered":
		ord := order.New(db)
		trackingRsrc := trackingMsg.Body.Resource
		if err := ord.GetById(trackingRsrc.OrderExternalId); err != nil {
			http.Fail(c, 400, "Failed decode request body", err)
			return
		}

		ord.Fulfillment.TrackingNumber = trackingRsrc.Tracking
		if !trackingRsrc.LabelCreatedDate.IsZero() {
			ord.FulfillmentStatus = FulfillmentLabelled
		} else if !trackingRsrc.TrackedDate.IsZero() {
			ord.FulfillmentStatus = FulfillmentProcessing
		} else if !trackingRsrc.FirstScanDate.IsZero() {
			ord.FulfillmentStatus = FulfillmentShipped
		} else if !trackingRsrc.DeliveredDate.IsZero() {
			ord.FulfillmentStatus = FulfillmentDelivered
		}
		ord.Fulfillment.CreatedAt = trackingRsrc.LabelCreatedDate
		ord.Fulfillment.ShippedAt = trackingRsrc.FirstScanDate
		ord.Fulfillment.DeliveredAt = trackingRsrc.DeliveredDate
		// ord.Fulfillment.Service = req.Service
		ord.Fulfillment.Carrier = trackingRsrc.Carrier

		// usr := user.New(db)
		// usr.MustGetById(ord.UserId)

		// pay := payment.New(db)
		// pay.MustGetById(ord.PaymentIds[0])

		// emails.SendFulfillmentEmail(db.Context, org, ord, usr, pay)
		ord.MustPut()
	}

	c.String(200, "ok\n")
}
