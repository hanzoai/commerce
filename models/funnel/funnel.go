package funnel

import (
	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/util/json"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Funnel struct {
	mixin.Model

	Name    string     `json:"name"`
	Events  [][]string `json:"events" datastore:"-"`
	Events_ string     `json:"-"`
}

func (f *Funnel) Init() {
	f.Events = make([][]string, 0)
}

func New(db *datastore.Datastore) *Funnel {
	f := new(Funnel)
	f.Init()
	f.Model = mixin.Model{Db: db, Entity: f}
	return f
}

func (f Funnel) Kind() string {
	return "funnel"
}

func (f Funnel) Document() mixin.Document {
	return nil
}

func (f *Funnel) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	f.Init()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(f, c)); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(f.Events_) > 0 {
		err = json.DecodeBytes([]byte(f.Events_), &f.Events)
	}

	return
}

func (f *Funnel) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	f.Events_ = string(json.EncodeBytes(&f.Events))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(f, c))
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
