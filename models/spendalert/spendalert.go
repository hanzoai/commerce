package spendalert

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/val"
	"github.com/hanzoai/orm"
)

func init() { orm.Register[SpendAlert]("spend-alert") }

// SpendAlert is a user-configured threshold alert on cumulative spend.
// When the user's spend exceeds Threshold (in cents) for the billing period,
// the alert triggers (TriggeredAt is set).
type SpendAlert struct {
	mixin.Model[SpendAlert]

	UserId    string `json:"userId"`
	Title     string `json:"title"`
	Threshold int64  `json:"threshold"` // cents
	Currency  string `json:"currency"`

	// TriggeredAt stores the ISO timestamp when the alert last fired.
	// Empty string means not yet triggered.
	TriggeredAt string `json:"triggeredAt,omitempty"`
}

func (s *SpendAlert) Validator() *val.Validator {
	return val.New()
}

func New(db *datastore.Datastore) *SpendAlert {
	s := new(SpendAlert)
	s.Init(db)
	s.Parent = db.NewKey("synckey", "", 1, nil)
	return s
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("spend-alert")
}
