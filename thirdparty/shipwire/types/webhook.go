package types

import (
	"encoding/json"
	"time"
)

// Webhook Responses
type Message struct {
	Topic                 string    `json:"topic"`
	Attempt               int       `json:"attempt"`
	Timestamp             time.Time `json:"timestamp"`
	UniqueEventId         string    `json:"uniqueEventID"`
	WebhookSubscriptionId int       `json:"webhookSubscriptionID"`
	Body                  Body      `json:"body"`
}

// Webhook Response Bodies
type Body struct {
	Status           int             `json:"status"`
	Message          string          `json:"message"`
	Resource         json.RawMessage `json:"resource"`
	ResourceLocation string          `json:"resourceLocation"`
}
