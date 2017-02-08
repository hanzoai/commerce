package subscriber

import (
	"strings"
	"time"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/client"
	"hanzo.io/util/json"

	. "hanzo.io/models"
	. "hanzo.io/util/strings"
)

type Subscriber struct {
	mixin.Model

	Email     string `json:"email"`
	FormId    string `json:"formId,omitempty"`
	UserId    string `json:"userId"`
	SegmentId string `json:"segmentId"`

	Unsubscribed    bool      `json:"unsubscribed"`
	UnsubscribeDate time.Time `json:"unsubscribeDate,omitempty"`

	Client client.Client `json:"client"`

	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (s *Subscriber) Defaults() {
	s.Metadata = make(Map)
}

func (s Subscriber) MergeVars() Map {
	vars := make(Map)

	for k, v := range s.Metadata {
		vars[k] = v
	}

	// Update metadata with some extra client data
	vars["useragent"] = s.Client.UserAgent
	vars["referer"] = s.Client.Referer
	vars["language"] = s.Client.Language
	vars["country"] = s.Client.Country
	vars["region"] = s.Client.Region
	vars["city"] = s.Client.City

	return vars
}

func (s *Subscriber) Normalize() {
	s.Email = StripWhitespace(s.Email)
	s.Email = strings.ToLower(s.Email)
}

func FromJSON(db *datastore.Datastore, data []byte) *Subscriber {
	s := New(db)
	json.DecodeBytes(data, s)
	return s
}
