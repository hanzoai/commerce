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
	Name     string `json:"-"`
	Disabled bool   `json:"-"`

	// Common to all integrations
	Type  string `json:"type"`
	Event string `json:"event,omitempty"`
	Id    string `json:"id,omitempty"`

	Src struct {
		Url  string `json:"url,omitempty"`
		Type string `json:"type,omitempty"`
	} `json:"src,omitempty"`

	// Available integrations
	Generic
	FacebookAudiences
	FacebookConversions
	GoogleAnalytics
	GoogleAdWords
}

// Integration specific properties
type Generic struct {
	Code string `json:"code,omitempty"`
}

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
