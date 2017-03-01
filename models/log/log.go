package log_

import (
	"hanzo.io/models/mixin"
	"time"
)

type Log struct {
	mixin.Model

	Enabled bool `json:"enabled"`

	Time    time.Time `json:"time"`
	Source  string    `json:"source"`
	Message string    `json:"message"`
}
