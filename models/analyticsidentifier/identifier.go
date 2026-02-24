package analyticsidentifier

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/orm"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

func init() { orm.Register[AnalyticsIdentifier]("analyticsidentifier") }

type Ids struct {
	// Unique Identifier set on the client to identify the browser + device
	UUId string `json:"uuid"`

	// User Id
	UserId string `json:"userId"`

	// Google Analytics Long Id
	GAId string `json:"ga"`

	// Facebook Id
	FBId string `json:"fr"`
}

type AnalyticsIdentifier struct {
	mixin.Model[AnalyticsIdentifier]
	Ids
}

// New creates a new AnalyticsIdentifier wired to the given datastore.
func New(db *datastore.Datastore) *AnalyticsIdentifier {
	e := new(AnalyticsIdentifier)
	e.Init(db)
	return e
}

// Query returns a datastore query for analytics identifiers.
func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("analyticsidentifier")
}
