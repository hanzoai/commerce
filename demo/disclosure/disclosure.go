package disclosure

import (
	"hanzo.io/models/mixin"
)

// Datastructure for Bitcoin Transaction
type Disclosure struct {
	mixin.Model

	// The searchable module we use is called 'document' so this has to be
	// called something else.
	Publication string `json:"publication"`
	Hash        string `json:"hash"`
	Type        string `json:"type"`
	Receiver    string `json:"receiver"`
}
