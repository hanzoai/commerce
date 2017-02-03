package webhook

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/thirdparty/shipwire/response"
)

// Webhook Response Body
type MessageBody struct {
	Body struct {
		response.Ref

		Status  string `json:"status"`
		Message string `json:"message"`
	}
}

// Webhook Response
type Message struct {
	Topic                 string      `json:"topic"`
	Attempt               string      `json:"attempt"`
	Timestamp             time.Time   `json:"timestamp"`
	UniqueEventId         string      `json:"uniqueEventID"`
	WebhookSubscriptionId int         `json:"webhookSubscriptionID"`
	Body                  MessageBody `json:"body"`
}

func Process(c *gin.Context) {
	c.String(200, "ok\n")
}
