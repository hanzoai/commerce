package analytics

import (
	"crowdstart.com/util/json"
)

var t bool
var f bool

func init() {
	t = true
	f = false
}

type Analytics struct {
	Integrations []Integration `json:"integrations,omitempty"`
}

func (a Analytics) UpdateStoredDisabledStatus() Analytics {
	for i, integration := range a.Integrations {
		if integration.Disabled_ == nil {
			a.Integrations[i].Disabled = false
			continue
		}

		a.Integrations[i].Disabled = *integration.Disabled_
	}

	return a
}

func (a Analytics) UpdateShownDisabledStatus() Analytics {
	for i, integration := range a.Integrations {
		if integration.Disabled {
			a.Integrations[i].Disabled_ = &t
		} else {
			a.Integrations[i].Disabled_ = &f
		}
	}

	return a
}

// Only JSON encode enabled integrations
func (a Analytics) SnippetJSON() []byte {
	enabled := Analytics{}
	for _, integration := range a.Integrations {
		if !integration.Disabled {
			integration.Disabled_ = nil
			enabled.Integrations = append(enabled.Integrations, integration)
		}
	}

	return json.EncodeBytes(enabled)
}

type Integration struct {
	Name      string `json:"-"`
	Disabled  bool   `json:"-"`
	Disabled_ *bool  `json:"disabled,omitempty" datastore:"-"`

	// Common to all integrations
	Type  string `json:"type"`
	Event string `json:"event,omitempty"`
	Id    string `json:"id,omitempty"`

	Src struct {
		Url  string `json:"url,omitempty"`
		Type string `json:"type,omitempty"`
	} `json:"src,omitempty"`

	// Available integrations
	Custom
	FacebookAudiences
	FacebookConversions
	GoogleAnalytics
	GoogleAdWords
}

// Integration specific properties
type Custom struct {
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
