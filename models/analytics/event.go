package analytics

import (
	"time"

	"hanzo.io/models/mixin"
	"hanzo.io/models/types/client"

	. "hanzo.io/models"
)

type AnalyticsEvent struct {
	mixin.Model

	UserId     string `json:"userId"`
	SessionId  string `json:"sessionId"`
	PageId     string `json:"pageId"`
	PageViewId string `json:"pageViewId"`

	UAString            string    `json:"uaString"`
	UA                  UserAgent `json:"ua"`
	Timestamp           time.Time `json:"timestamp"`
	CalculatedTimestamp time.Time `json:"-"`

	Name            string        `json:"name"` // Event appended with special data (used by pageview and pageleave)
	Event           string        `json:"event"`
	Data            Map           `json:"data" datastore:"-"`
	Data_           string        `json:"-" datastore:",noindex"`
	Count           int           `json:"count"`
	RequestMetadata client.Client `json:"-"`
}

func (e *AnalyticsEvent) Defaults() {
	e.Data = make(Map)
}
