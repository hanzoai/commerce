package analyticsevent

import (
	"time"

	aeds "appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/analyticsidentifier"
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/client"
	"hanzo.io/util/json"

	. "hanzo.io/models"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type UserAgent struct {
	Browser struct {
		Name    string
		Version string
	}
	Engine struct {
		Name    string
		Version string
	}
	Os struct {
		Name    string
		Version string
	}
	Device struct {
		Model  string
		Type   string
		Vendor string
	}
	Cpu struct {
		Architecture string
	}
}

type AnalyticsEvent struct {
	mixin.Model

	analyticsidentifier.Ids

	SessionId  string `json:"sessionId"`
	PageId     string `json:"pageId"`
	PageViewId string `json:"pageViewId"`

	UAString string    `json:"uaString"`
	UA       UserAgent `json:"ua"`

	Timestamp           time.Time `json:"timestamp"`
	CalculatedTimestamp time.Time `json:"-"`

	Name            string        `json:"name"` // Event appended with special data (used by pageview and pageleave)
	Event           string        `json:"event"`
	Data            Map           `json:"data" datastore:"-"`
	Data_           string        `json:"-" datastore:",noindex"`
	RequestMetadata client.Client `json:"requestMetadata"`
}

func (e *AnalyticsEvent) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	e.Defaults()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(e, c)); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(e.Data_) > 0 {
		err = json.DecodeBytes([]byte(e.Data_), &e.Data)
	}

	return
}

func (e *AnalyticsEvent) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	e.Data_ = string(json.EncodeBytes(&e.Data))

	e.Name = e.Event
	if e.Event == "PageView" || e.Event == "PageLeave" {
		e.Name += "_" + e.PageId
	}

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(e, c))
}
