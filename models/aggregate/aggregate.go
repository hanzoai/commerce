package aggregate

import (
	"strconv"
	"time"

	"hanzo.io/models/mixin"
)

type Frequency string

const (
	Hourly Frequency = "Hourly"
	Daily            = "Daily"
)

type Aggregate struct {
	mixin.Model

	Instance     string    `json:"instance"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	BinTimestamp time.Time `json:"binTimestamp"`
	Value        int64     `json:"value"`
	VectorValue  []int64   `json:"vectorValue,omitempty"`
}

func (a *Aggregate) Defaults() {
	a.VectorValue = make([]int64, 0)
}

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
