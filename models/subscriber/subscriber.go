package subscriber

import (
	"time"

	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/util/json"
	"crowdstart.com/util/val"

	. "crowdstart.com/models"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Subscriber struct {
	mixin.Model

	Email         string `json:"email"`
	MailingListId string `json:"mailingListId"`
	UserId        string `json:"userId,omitempty"`

	Unsubscribed    bool      `json:"unsubscribed"`
	UnsubscribeDate time.Time `json:"unsubscribeDate,omitempty"`

	Metadata  Metadata `json:"metadata" datastore:"-"`
	Metadata_ string   `json:"-" datastore:",noindex"`
}

func (s *Subscriber) Init() {
	s.Metadata = make(Metadata)
}

func New(db *datastore.Datastore) *Subscriber {
	s := new(Subscriber)
	s.Init()
	s.Model = mixin.Model{Db: db, Entity: s}
	return s
}

func (s Subscriber) Key() string {
	return s.MailingListId + ":" + s.Email
}

func (s Subscriber) Kind() string {
	return "subscriber"
}

func (s Subscriber) Document() mixin.Document {
	return nil
}

func (s *Subscriber) Load(c <-chan aeds.Property) (err error) {
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

func (s *Subscriber) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	s.Metadata_ = string(json.EncodeBytes(&s.Metadata))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(s, c))
}

func (s *Subscriber) Validator() *val.Validator {
	return val.New(s)
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}

func FromJSON(db *datastore.Datastore, data []byte) *Subscriber {
	s := New(db)
	json.DecodeBytes(data, s)
	return s
}
