package datastore

import (
	"appengine"
	. "appengine/datastore"

	"github.com/gin-gonic/gin"
	"github.com/qedus/nds"

	"crowdstart.io/util/log"
)

type Datastore struct {
	Context appengine.Context
}

func New(ctx interface{}) (d *Datastore) {
	switch ctx := ctx.(type) {
	case appengine.Context:
		d = &Datastore{ctx}
	case *gin.Context:
		c := ctx.MustGet("appengine").(appengine.Context)
		d = &Datastore{c}
	}
	return d
}

func (d *Datastore) Get(key string, value interface{}) error {
	k, err := DecodeKey(key)
	if err != nil {
		log.Error("%v", err, d.Context)
		return err
	}

	err = nds.Get(d.Context, k, value)
	if err != nil {
		log.Error("%v", err, d.Context)
	}
	return err
}

func (d *Datastore) GetKey(kind, key string, value interface{}) error {
	k := NewKey(d.Context, kind, key, 0, nil)
	err := nds.Get(d.Context, k, value)
	if err != nil {
		log.Error("%v, %v, %v", kind, key, err, d.Context)
	}
	return err
}

func (d *Datastore) GetMulti(keys []string, values []interface{}) error {
	_keys := make([]*Key, len(keys))

	for _, v := range keys {
		if key, err := DecodeKey(v); err != nil {
			d.Context.Errorf("%#v", err)
			return err
		} else {
			_keys = append(_keys, key)
		}
	}

	return nds.GetMulti(d.Context, _keys, values)
}

func (d *Datastore) GetKeyMulti(kind string, keys []string, values []interface{}) error {
	_keys := make([]*Key, len(keys))

	for _, v := range keys {
		_keys = append(_keys, NewKey(d.Context, kind, v, 0, nil))
	}

	return nds.GetMulti(d.Context, _keys, values)
}

func (d *Datastore) Put(kind string, src interface{}) (string, error) {
	k := NewIncompleteKey(d.Context, kind, nil)
	k, err := nds.Put(d.Context, k, src)
	if err != nil {
		d.Context.Errorf("%#v", err)
		return "", err
	}
	return k.Encode(), nil
}

func (d *Datastore) PutKey(kind string, key string, src interface{}) (string, error) {
	k := NewKey(d.Context, kind, key, 0, nil)
	k, err := nds.Put(d.Context, k, src)
	if err != nil {
		d.Context.Errorf("%#v", err)
		return "", err
	}
	return k.Encode(), nil
}

func (d *Datastore) PutMulti(kind string, srcs []interface{}) (keys []string, err error) {
	nkeys := len(srcs)
	_keys := make([]*Key, nkeys)

	for i := 0; i < nkeys; i++ {
		_keys[i] = NewIncompleteKey(d.Context, kind, nil)
	}

	_keys, err = nds.PutMulti(d.Context, _keys, srcs)
	if err != nil {
		d.Context.Errorf("%#v", err)
		return keys, err
	}

	keys = make([]string, nkeys)
	for i := 0; i < nkeys; i++ {
		keys[i] = _keys[i].Encode()
	}

	return keys, nil
}

func (d *Datastore) PutKeyMulti(kind string, keys []string, srcs []interface{}) ([]string, error) {
	nkeys := len(srcs)
	_keys := make([]*Key, nkeys)

	for i := 0; i < nkeys; i++ {
		_keys[i] = NewKey(d.Context, kind, keys[i], 0, nil)
	}

	_keys, err := nds.PutMulti(d.Context, _keys, srcs)
	if err != nil {
		d.Context.Errorf("%#v", err)
		return keys, err
	}

	for i := 0; i < nkeys; i++ {
		keys[i] = _keys[i].Encode()
	}

	return keys, nil
}

func (d *Datastore) Update(key string, src interface{}) (string, error) {
	log.Warn("DEPRECATED. DOES NOTHING PUT DOES NOT.")

	k, err := DecodeKey(key)
	if err != nil {
		d.Context.Errorf("%#v", err)
		return "", err
	}

	k, err = nds.Put(d.Context, k, src)
	if err != nil {
		d.Context.Errorf("%#v", err)
		return "", err
	}
	return k.Encode(), nil
}

func (d *Datastore) Delete(key string) error {
	k, err := DecodeKey(key)
	if err != nil {
		d.Context.Errorf("%#v", err)
		return err
	}
	return nds.Delete(d.Context, k)
}

func (d *Datastore) Query(kind string) *Query {
	return NewQuery(kind)
}

func (d *Datastore) RunInTransaction(f func(tc appengine.Context) error, opts *TransactionOptions) error {
	return nds.RunInTransaction(d.Context, f, opts)
}
