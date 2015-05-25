package referral

import (
	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/util/val"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Referral struct {
	mixin.Model

	Triggers []int     `json:"triggers"`
	Actions  []float64 `json:"actions"`
}

func New(db *datastore.Datastore) *Referral {
	r := new(Referral)
	r.Model = mixin.Model{Db: db, Entity: r}
	return r
}

func (r Referral) Kind() string {
	return "referral"
}

func (r *Referral) Load(c <-chan aeds.Property) (err error) {
	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(r, c)); err != nil {
		return err
	}

	return err
}

func (r *Referral) Save(c chan<- aeds.Property) (err error) {
	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(r, c))
}

func (r *Referral) Validator() *val.Validator {
	return nil
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
