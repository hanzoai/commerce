package submission

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/client"
	"hanzo.io/util/json"

	. "hanzo.io/models"
)

type Submission struct {
	mixin.Model

	Email  string `json:"email"`
	UserId string `json:"userId,omitempty"`

	Client client.Client `json:"client"`

	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (s *Submission) Defaults() {
	s.Metadata = make(Map)
}

func FromJSON(db *datastore.Datastore, data []byte) *Submission {
	s := New(db)
	json.DecodeBytes(data, s)
	return s
}
