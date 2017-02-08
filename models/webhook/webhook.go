package webhook

import "hanzo.io/models/mixin"

type Events map[string]bool

type Webhook struct {
	mixin.Model

	// Endpoint webhook should deliver events to.
	Url string `json:"url"`

	// Whether to use Live or Test data.
	Live bool `json:"live"`

	// Whether to send all events or selectively using Events.
	All bool `json:"all"`

	// Events to selectively send.
	Events  Events `json:"events" datastore:"-"`
	Events_ string `json:"-" datastore:",noindex"`

	// Whether this webhook is enabled or not.
	Enabled bool `json:"enabled"`
}

func (w *Webhook) Defaults() {
	w.Events = make(Events)
}
