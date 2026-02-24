package analyticsevent

import (
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/analyticsidentifier"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/client"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/orm"

	. "github.com/hanzoai/commerce/types"
)

func init() { orm.Register[AnalyticsEvent]("analyticsevent") }

type AnalyticsEvent struct {
	mixin.EntityBridge[AnalyticsEvent]

	analyticsidentifier.Ids

	SessionId  string `json:"sessionId"`
	PageId     string `json:"pageId"`
	PageViewId string `json:"pageViewId"`

	Timestamp           time.Time `json:"timestamp"`
	CalculatedTimestamp time.Time `json:"-"`

	Name            string        `json:"name"` // Event appended with special data (used by pageview and pageleave)
	Event           string        `json:"event"`
	Data            Map           `json:"data" datastore:"-" orm:"default:{}"`
	Data_           string        `json:"-" datastore:",noindex"`
	RequestMetadata client.Client `json:"requestMetadata"`
}

func (e *AnalyticsEvent) Load(ps []datastore.Property) (err error) {
	// Ensure we're initialized
	if e.Data == nil {
		e.Data = make(Map)
	}

	// Load supported properties
	if err = datastore.LoadStruct(e, ps); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(e.Data_) > 0 {
		err = json.DecodeBytes([]byte(e.Data_), &e.Data)
	}

	return err
}

func (e *AnalyticsEvent) Save() (ps []datastore.Property, err error) {
	// Serialize unsupported properties
	e.Data_ = string(json.EncodeBytes(&e.Data))

	e.Name = e.Event
	if e.Event == "PageView" || e.Event == "PageLeave" {
		e.Name += "_" + e.PageId
	}

	// Save properties
	return datastore.SaveStruct(e)
}

func New(db *datastore.Datastore) *AnalyticsEvent {
	e := new(AnalyticsEvent)
	e.Init(db)
	return e
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("analyticsevent")
}
