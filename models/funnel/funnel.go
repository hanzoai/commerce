package funnel

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/orm"
)

func init() { orm.Register[Funnel]("funnel") }

type Funnel struct {
	mixin.Model[Funnel]

	Name    string     `json:"name"`
	Events  [][]string `json:"events" datastore:"-"`
	Events_ string     `json:"-"`
}

func (f *Funnel) Load(ps []datastore.Property) (err error) {
	// Ensure we're initialized
	f.Defaults()

	// Load supported properties
	if err = datastore.LoadStruct(f, ps); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(f.Events_) > 0 {
		err = json.DecodeBytes([]byte(f.Events_), &f.Events)
	}

	return
}

func (f *Funnel) Save() (ps []datastore.Property, err error) {
	// Serialize unsupported properties
	f.Events_ = string(json.EncodeBytes(&f.Events))

	// Save properties
	return datastore.SaveStruct(f)
}

func (f *Funnel) Defaults() {
	f.Events = make([][]string, 0)
}

func New(db *datastore.Datastore) *Funnel {
	f := new(Funnel)
	f.Init(db)
	f.Defaults()
	return f
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("funnel")
}
