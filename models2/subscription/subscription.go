package subscription

import (
	"time"

	aeds "appengine/datastore"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/json"
	"crowdstart.io/util/val"

	. "crowdstart.io/models2"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Subscription struct {
	mixin.Model

	Email         string `json:"email"`
	MailingListId string `json:"mailingListId"`
	UserId        string `json:"userId,omitempty"`

	Unsubscribed    bool      `json:"unsubscribed"`
	UnsubscribeDate time.Time `json:"unsubscribeDate,omitempty"`

	Metadata  Metadata `json:"metadata" datastore:"-"`
	Metadata_ string   `json:"-" datastore:",noindex"`
}

func (s *Subscription) Init() {
	s.Metadata = make(Metadata)
}

func New(db *datastore.Datastore) *Subscription {
	s := new(Subscription)
	s.Init()
	s.Model = mixin.Model{Db: db, Entity: s}
	return s
}

func (s Subscription) Key() string {
	return s.MailingListId + ":" + s.Email
}

func (s Subscription) Kind() string {
	return "subscription"
}

func (s *Subscription) Load(c <-chan aeds.Property) (err error) {
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

func (s *Subscription) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	s.Metadata_ = string(json.EncodeBytes(&s.Metadata))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(s, c))
}

func (s *Subscription) Validator() *val.Validator {
	return val.New(s)
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
