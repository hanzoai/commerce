package funnel

import "hanzo.io/models/mixin"

type Funnel struct {
	mixin.Model

	Name    string     `json:"name"`
	Events  [][]string `json:"events" datastore:"-"`
	Events_ string     `json:"-"`
}

func (f *Funnel) Defaults() {
	f.Events = make([][]string, 0)
}
