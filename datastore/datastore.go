package datastore

import (
	"appengine"
	. "appengine/datastore"
	"github.com/qedus/nds"
)

type Datastore struct {
	ctx appengine.Context
}

func New(ctx appengine.Context) *Datastore {
	return &Datastore{ctx}
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
