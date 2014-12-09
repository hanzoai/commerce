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

func (d *Datastore) DecodeKey(encodedKey string) (*Key, error) {
	key, err := DecodeKey(encodedKey)
	if err != nil {
		log.Warn("Unable to decode key: %v", encodedKey)
	}
	return key, err
}

func (d *Datastore) Get(key string, value interface{}) error {
	k, err := DecodeKey(key)
	if err != nil {
		return err
	}

	err = nds.Get(d.Context, k, value)
	if _, ok := err.(*ErrFieldMismatch); ok {
		// Ignore any field mismatch errors.
		log.Warn("Field mismatch when getting %v: %v", key, err, d.Context)
		err = nil
	} else {
		log.Warn("Failed to get %v: %v", key, err, d.Context)
	}
	return err
}

func (d *Datastore) GetKey(kind, key string, value interface{}) error {
	k := NewKey(d.Context, kind, key, 0, nil)
	if err := nds.Get(d.Context, k, value); err != nil {
		if _, ok := err.(*ErrFieldMismatch); ok {
			// Ignore any field mismatch errors.
			log.Warn("Field mismatch when getting kind %v, key %v: %v", kind, key, err, d.Context)
			err = nil
		} else {
			log.Warn("Failed to get kind %v, key %v: %v", kind, key, err, d.Context)
			return err
		}
	}
	return nil
}

func (d *Datastore) GetMulti(keys []string, vals interface{}) error {
	_keys := make([]*Key, len(keys))

	for i, key := range keys {
		if k, err := DecodeKey(key); err != nil {
			log.Warn("%v", err, d.Context)
			return err
		} else {
			_keys[i] = k
		}
	}

	return nds.GetMulti(d.Context, _keys, vals)
}

func (d *Datastore) GetKeyMulti(kind string, keys []string, vals interface{}) error {
	_keys := make([]*Key, len(keys))

	for i, key := range keys {
		_keys[i] = NewKey(d.Context, kind, key, 0, nil)
	}

	return nds.GetMulti(d.Context, _keys, vals)
}

func (d *Datastore) Put(kind string, src interface{}) (string, error) {
	k := NewIncompleteKey(d.Context, kind, nil)
	k, err := nds.Put(d.Context, k, src)
	if err != nil {
		log.Warn("%v", err, d.Context)
		return "", err
	}
	return k.Encode(), nil
}

func (d *Datastore) PutKey(kind string, key interface{}, src interface{}) (string, error) {
	var k *Key
	switch v := key.(type) {
	case string:
		k = NewKey(d.Context, kind, v, 0, nil)
	case *Key:
		k = v
	}

	k, err := nds.Put(d.Context, k, src)
	if err != nil {
		log.Warn("%v, %v, %v, %#v", err, kind, k, src, d.Context)
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
		log.Warn("%v", err, d.Context)
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
		log.Warn("%v", err, d.Context)
		return keys, err
	}

	for i := 0; i < nkeys; i++ {
		keys[i] = _keys[i].Encode()
	}

	return keys, nil
}

func (d *Datastore) Update(key string, src interface{}) (string, error) {
	log.Warn("DEPRECATED. DOES NOTHING PUT DOES NOT.", d.Context)

	k, err := DecodeKey(key)
	if err != nil {
		log.Warn("%v", err, d.Context)
		return "", err
	}

	k, err = nds.Put(d.Context, k, src)
	if err != nil {
		log.Warn("%v", err, d.Context)
		return "", err
	}
	return k.Encode(), nil
}

func (d *Datastore) Delete(key string) error {
	k, err := DecodeKey(key)
	if err != nil {
		log.Warn("%v", err, d.Context)
		return err
	}
	return nds.Delete(d.Context, k)
}

func (d *Datastore) DeleteMulti(keys []string) error {
	_keys := make([]*Key, 0)
	for _, key := range keys {
		k, err := DecodeKey(key)
		_keys = append(_keys, k)
		if err != nil {
			log.Warn("%v", err, d.Context)
			return err
		}
	}
	return nds.DeleteMulti(d.Context, _keys)
}

func (d *Datastore) Query(kind string) *Query {
	return NewQuery(kind)
}

func (d *Datastore) RunInTransaction(f func(tc appengine.Context) error, opts *TransactionOptions) error {
	return nds.RunInTransaction(d.Context, f, opts)
}
