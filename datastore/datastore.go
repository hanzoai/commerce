package datastore

import (
	"appengine"
	. "appengine/datastore"
	"github.com/gin-gonic/gin"
	"github.com/qedus/nds"
	"crowdstart.io/middleware"
)

type Datastore struct {
	ctx appengine.Context
}

func New(ctx interface{}) (d *Datastore) {
	switch ctx := ctx.(type) {
	case appengine.Context:
		d = &Datastore{ctx}
	case *gin.Context:
		c := middleware.GetAppEngine(ctx)
		d = &Datastore{c}
	}
	return d
}

func (d *Datastore) Get(key string, value interface{}) error {
	k, err := DecodeKey(key)
	if err != nil {
		return err
	}

	return nds.Get(d.ctx, k, value)
}

func (d *Datastore) Put(kind string, src interface{}) (string, error) {
	k := NewIncompleteKey(d.ctx, kind, nil)
	k, err := nds.Put(d.ctx, k, src)
	if err != nil {
		return "", err
	}
	return k.Encode(), nil
}

func (d *Datastore) Update(key string, src interface{}) (string, error) {
	k, err := DecodeKey(key)
	if err != nil {
		return "", err
	}

	k, err = nds.Put(d.ctx, k, src)
	if err != nil {
		return "", err
	}
	return k.Encode(), nil
}

func (d *Datastore) Delete(key string) error {
	k, err := DecodeKey(key)
	if err != nil {
		return err
	}
	return nds.Delete(d.ctx, k)
}

func (d *Datastore) RunInTransaction(f func(tc appengine.Context) error, opts *TransactionOptions) error {
	return nds.RunInTransaction(d.ctx, f, opts)
}
