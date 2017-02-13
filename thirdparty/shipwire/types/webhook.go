package types

import (
	"hanzo.io/util/json"
	"time"
)

// Webhook Responses
type Message struct {
	Topic                 string      `json:"topic"`
	Attempt               string      `json:"attempt"`
	Timestamp             time.Time   `json:"timestamp"`
	UniqueEventId         string      `json:"uniqueEventID"`
	WebhookSubscriptionId int         `json:"webhookSubscriptionID"`
	Body                  MessageBody `json:"body"`
}

// Webhook Response Bodies
type MessageBody struct {
	Status           string          `json:"status"`
	Message          string          `json:"message"`
	Resource         json.RawMessage `json:"resource"`
	ResourceLocation string          `json:"resourceLocation"`
}
