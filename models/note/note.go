package note

import (
	"hanzo.io/models/mixin"
	"time"
)

type Note struct {
	mixin.Model

	Enabled bool `json:"enabled"`

	Time    time.Time `json:"time"`
	Source  string    `json:"source"`
	Message string    `json:"message"`
}
