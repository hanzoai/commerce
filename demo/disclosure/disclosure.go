package disclosure

import (
	"hanzo.io/models/mixin"
	"time"
)

// Datastructure for Bitcoin Transaction
type Transaction struct {
	mixin.Model

	Document  string    `json:"document"`
	Type      string    `json:"type"`
	Receiver  string    `json:"receiver"`
	Timestamp time.Time `json:"timestamp"`
}
