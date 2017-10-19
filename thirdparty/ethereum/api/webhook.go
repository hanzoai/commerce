package api

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/util/log"

	. "hanzo.io/models/blockchains"
)

type Kind string

const (
	BlockKind        Kind = "block"
	BlockAddress     Kind = "blockaddress"
	BlockTransaction Kind = "blocktransaction"
)

type Event struct {
	Id       string `json:"id"`
	Kind     Kind   `json:"kind"`
	Type     Type   `json:"type"`
	Password string `json:"password"`

	Data map[string]interface{} `json:"data"`
}

// Decode Ethereum payload
func decodeEvent(c *gin.Context) (*Event, error) {
	event := new(Event)
	if err := json.Decode(c.Request.Body, event); err != nil {
		log.Error("Could not Decode:\n%s", c.Request.Body, c)
		return nil, fmt.Errorf("Failed to parse Stripe webhook: %v", err)
	}

	log.JSON("Received '%s'", event.Type, event)
	return event, nil
}

var AccessDeniedError = errors.New("Access Denied")

// Handle stripe webhook POSTs
func Webhook(c *gin.Context) {
	event, err := decodeEvent(c)
	if err != nil {
		http.Fail(c, 500, err.Error(), err)
		return
	}

	if event.Password != config.Ethereum.WebhookPassword {
		http.Fail(c, 401, AccessDeniedError.Error(), AccessDeniedError)
		return
	}

	log.Info("Received Ethereum Webhook: %v", event, c)
}
