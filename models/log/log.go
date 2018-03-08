package log

import (
	"hanzo.io/models/mixin"

	. "hanzo.io/models"
)

type Log struct {
	mixin.Model

	Creator string   `json:"creator"`
	Message string   `json:"message"`
	Tags    []string `json:"tags"`
	Tags_   string   `json:"-"` // need props

	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (l *Log) Defaults() {
	l.Metadata = make(Map)
}
