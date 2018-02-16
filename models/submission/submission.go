package submission

import (
	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/client"
	"hanzo.io/util/json"
	"hanzo.io/util/val"

	. "hanzo.io/models"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Submission struct {
	mixin.Model

	Email  string `json:"email"`
	UserId string `json:"userId,omitempty"`

	Client client.Client `json:"client"`

	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (s *Submission) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	s.Defaults()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(s, c)); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(s.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(s.Metadata_), &s.Metadata)
	}

	return err
}

func (s *Submission) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	s.Metadata_ = string(json.EncodeBytes(&s.Metadata))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(s, c))
}

func (s *Submission) Validator() *val.Validator {
	return val.New()
}

func FromJSON(db *datastore.Datastore, data []byte) *Submission {
	s := New(db)
	json.DecodeBytes(data, s)
	return s
}
