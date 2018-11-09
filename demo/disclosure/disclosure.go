package disclosure

import (
	"hanzo.io/models/mixin"
	"time"
)

// Datastructure for Bitcoin Transaction
type Disclosure struct {
	mixin.Model

	// The searchable module we use is called 'document' so this has to be
	// called something else.
	Publication string    `json:"publication"`
	Type        string    `json:"type"`
	Receiver    string    `json:"receiver"`
	Timestamp   time.Time `json:"timestamp"`
}
