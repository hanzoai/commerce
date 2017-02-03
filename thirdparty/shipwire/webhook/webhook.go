package webhook

import (
	"time"

	"../response"
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
