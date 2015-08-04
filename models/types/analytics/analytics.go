package analytics

import (
	"crowdstart.com/util/json"
)

type Analytics struct {
	Integrations []Integration `json:"integrations,omitempty"`
}

// Only JSON encode enabled integrations
func (a Analytics) JSON() []byte {
	enabled := Analytics{}
	for _, integration := range a.Integrations {
		if !integration.Disabled {
			enabled.Integrations = append(enabled.Integrations, integration)
		}
	}

	return json.EncodeBytes(enabled)
}

type Integration struct {
	// Common to all integrations
	Name     string `json:"-"`
	Type     string `json:"type"`
	Disabled bool   `json:"-"`
	Id       string `json:"id,omitempty"`
	Event    string `json:"event,omitempty"`

	// Available integrations
	FacebookAudiences
	FacebookConversions
	GoogleAnalytics
	GoogleAdWords
}

// Integration specific properties
type FacebookAudiences struct {
}

type FacebookConversions struct {
	Value    string `json:"value,omitempty"`
	Currency string `json:"currency,omitempty"`
}

type GoogleAnalytics struct {
}

type GoogleAdWords struct {
}
