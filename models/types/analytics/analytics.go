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
	Id       string `json:"id,omitempty"`
	Event    string `json:"event,omitempty"`
	Type     string `json:"type"`
	Disabled bool   `json:"-"`

	// Embed every integration for easy customization
	FacebookAudiences
	FacebookConversions
	GoogleAnalytics
	GoogleAdWords
}

// Integration specific properties
type FacebookAudiences struct {
}

type FacebookConversions struct {
}

type GoogleAnalytics struct {
}

type GoogleAdWords struct {
}
