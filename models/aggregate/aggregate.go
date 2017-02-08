package aggregate

import (
	"strconv"
	"time"

	aeds "appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Frequency string

const (
	Hourly Frequency = "Hourly"
	Daily            = "Daily"
)

func Init(a *Aggregate, name string, t time.Time, freq Frequency) {
	switch freq {
	case Hourly:
		a.BinTimestamp = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	case Daily:
		a.BinTimestamp = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	default:
		a.BinTimestamp = t
	}
	a.Name = name
	a.Instance = name + "_" + string(freq) + "_" + strconv.Itoa(int(a.BinTimestamp.Unix()))
}

type Aggregate struct {
	mixin.Model

	Instance     string    `json:"instance"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	BinTimestamp time.Time `json:"binTimestamp"`
	Value        int64     `json:"value"`
	VectorValue  []int64   `json:"vectorValue,omitempty"`
}

func (a *Aggregate) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	a.Defaults()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(a, c)); err != nil {
		return err
	}

	return
}

func (a *Aggregate) Save(c chan<- aeds.Property) (err error) {
	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(a, c))
}
