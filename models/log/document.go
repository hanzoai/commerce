package log

import (
	"strings"
	"time"

	"hanzo.io/models/mixin"
)

type Document struct {
	Id_ string

	CreatedAt time.Time
	UpdatedAt time.Time

	Creator string
	Message string
	Tags    string
}

func (d Document) Id() string {
	return string(d.Id_)
}

func (l Log) Document() mixin.Document {
	return &Document{
		l.Id(),
		l.CreatedAt,
		l.UpdatedAt,
		l.Creator,
		l.Message,
		strings.Join(l.Tags, " "),
	}
}
