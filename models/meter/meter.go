package meter

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/val"

	. "github.com/hanzoai/commerce/types"
)

// AggregationType controls how meter events are aggregated.
type AggregationType string

const (
	AggSum   AggregationType = "sum"
	AggCount AggregationType = "count"
	AggLast  AggregationType = "last"
)

// Meter defines a named usage metric with a specific aggregation strategy.
// Each meter has a unique EventName per org (e.g. "input_tokens", "api_calls").
type Meter struct {
	mixin.Model

	Name            string          `json:"name"`
	EventName       string          `json:"eventName"`
	AggregationType AggregationType `json:"aggregationType"`
	Currency        currency.Type   `json:"currency"`

	// JSON-encoded list of dimension names, e.g. ["model","region"]
	Dimensions  []string `json:"dimensions,omitempty" datastore:"-"`
	Dimensions_ string   `json:"-" datastore:",noindex"`

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (m *Meter) Load(ps []datastore.Property) (err error) {
	if err = datastore.LoadStruct(m, ps); err != nil {
		return err
	}

	if len(m.Dimensions_) > 0 {
		err = json.DecodeBytes([]byte(m.Dimensions_), &m.Dimensions)
		if err != nil {
			return err
		}
	}

	if len(m.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(m.Metadata_), &m.Metadata)
	}

	return err
}

func (m *Meter) Save() (ps []datastore.Property, err error) {
	m.Dimensions_ = string(json.EncodeBytes(&m.Dimensions))
	m.Metadata_ = string(json.EncodeBytes(&m.Metadata))
	return datastore.SaveStruct(m)
}

func (m *Meter) Validator() *val.Validator {
	return nil
}
