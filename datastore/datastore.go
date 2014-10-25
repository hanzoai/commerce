package datastore

import (
	"appengine"
	. "appengine/datastore"
	"github.com/gin-gonic/gin"
	"github.com/qedus/nds"
)

type Datastore struct {
	ctx appengine.Context
}

func New(ctx interface{}) (d *Datastore) {
	switch ctx := ctx.(type) {
	case appengine.Context:
		d = &Datastore{ctx}
	case gin.Context:
		c := ctx.MustGet("appengine").(appengine.Context)
		d = &Datastore{c}
	}
	return d
}

func (d *Datastore) NewKey(kind, stringID string, intID int64, parent *Key) *Key {
	return NewKey(d.ctx, kind, stringID, intID, parent)
}

func (d *Datastore) Get(key *Key, value interface{}) error {
	return nds.Get(d.ctx, key, value)
}

func (d *Datastore) Put(key *Key, src interface{}) (*Key, error) {
	return nds.Put(d.ctx, key, src)
}

func (d *Datastore) Delete(key *Key) error {
	return nds.Delete(d.ctx, key)
}

func (d *Datastore) RunInTransaction(f func(tc appengine.Context) error, opts *TransactionOptions) error {
	return nds.RunInTransaction(d.ctx, f, opts)
}
