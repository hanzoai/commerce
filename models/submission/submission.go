package submission

import (
	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/client"
	"crowdstart.com/util/json"
	"crowdstart.com/util/val"

	. "crowdstart.com/models"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Submission struct {
	mixin.Model

	Email  string `json:"email"`
	UserId string `json:"userId,omitempty"`

	Client client.Client `json:"client"`

	Metadata  Metadata `json:"metadata" datastore:"-"`
	Metadata_ string   `json:"-" datastore:",noindex"`
}

func (s *Submission) Init() {
	s.Metadata = make(Metadata)
}

func New(db *datastore.Datastore) *Submission {
	s := new(Submission)
	s.Init()
	s.Model = mixin.Model{Db: db, Entity: s}
	return s
}

func (s Submission) Kind() string {
	return "submission"
}

func (s *Submission) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	s.Init()

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
	return val.New(s)
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}

func FromJSON(db *datastore.Datastore, data []byte) *Submission {
	s := New(db)
	json.DecodeBytes(data, s)
	return s
}
