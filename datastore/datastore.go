package datastore

import (
	"errors"
	"strconv"

	"appengine"
	. "appengine/datastore"

	"github.com/gin-gonic/gin"
	"github.com/qedus/nds"

	"crowdstart.io/config"
	"crowdstart.io/util/log"
)

type Datastore struct {
	Context appengine.Context
	Warn    bool
}

func (d *Datastore) warn(fmtOrError interface{}, args ...interface{}) {
	if d.Warn {
		log.Warn(fmtOrError, args...)
	}
}

func New(ctx interface{}) (d *Datastore) {
	switch ctx := ctx.(type) {
	case appengine.Context:
		d = &Datastore{ctx, config.DatastoreWarn}
	case *gin.Context:
		c := ctx.MustGet("appengine").(appengine.Context)
		d = &Datastore{c, config.DatastoreWarn}
	}
	return d
}

func (d *Datastore) EncodeId(kind string, id interface{}) string {
	var _id int64
	switch v := id.(type) {
	case string:
		maybeId, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			d.warn("EncodeId was passed an string that could not be parsed to int64 %v", v)
			return ""
		}
		_id = maybeId
	case int64:
		_id = v
	case int:
		_id = int64(v)
	default:
		d.warn("EncodeId was passed an invalid type %v", v)
		return ""
	}

	return NewKey(d.Context, kind, "", _id, nil).Encode()
}

func (d *Datastore) DecodeKey(encodedKey string) (*Key, error) {
	_key, err := DecodeKey(encodedKey)

	// Since key returned might have been created with a different app, we'll
	// recreate the key to ensure it has a valid AppID.
	key := NewKey(d.Context, _key.Kind(), _key.StringID(), _key.IntID(), nil)

	if err != nil {
		d.warn("Unable to decode key: %v", encodedKey)
	}
	return key, err
}

// Gets an entity using an encoded key representation
func (d *Datastore) Get(key string, value interface{}) error {
	// Decode encoded key
	k, err := d.DecodeKey(key)
	if err != nil {
		return err
	}

	// Try to retrieve entity using nds, which transparently uses memcache if possible
	err = nds.Get(d.Context, k, value)
	if err != nil {
		if _, ok := err.(*ErrFieldMismatch); ok {
			// Ignore any field mismatch errors.
			d.warn("Field mismatch when getting %v: %v", key, err, d.Context)
			err = nil
		} else {
			d.warn("Failed to get %v: %v", key, err, d.Context)
		}
	}
	return err
}

// Gets an entity by literal datastore key of string type
func (d *Datastore) GetKey(kind, key string, value interface{}) error {
	// construct key manually using literal value and kind
	k := NewKey(d.Context, kind, key, 0, nil)

	// Try to retrieve entity using nds, which transparently uses memcache if possible
	if err := nds.Get(d.Context, k, value); err != nil {
		if _, ok := err.(*ErrFieldMismatch); ok {
			// Ignore any field mismatch errors.
			d.warn("Field mismatch when getting kind %v, key %v: %v", kind, key, err, d.Context)
			err = nil
		} else {
			d.warn("Failed to get kind %v, key %v: %v", kind, key, err, d.Context)
			return err
		}
	}
	return nil
}

func (d *Datastore) GetMulti(keys []string, vals interface{}) error {
	_keys := make([]*Key, len(keys))

	for i, key := range keys {
		if k, err := d.DecodeKey(key); err != nil {
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

// Puts entity, returning encoded key
func (d *Datastore) Put(kind string, src interface{}) (string, error) {
	k := NewIncompleteKey(d.Context, kind, nil)
	k, err := nds.Put(d.Context, k, src)
	if err != nil {
		d.warn("%v", err, d.Context)
		return "", err
	}
	return k.Encode(), nil
}

func (d *Datastore) PutKey(kind string, key interface{}, src interface{}) (string, error) {
	var k *Key
	switch v := key.(type) {
	case string:
		k = NewKey(d.Context, kind, v, 0, nil)
	case int64:
		k = NewKey(d.Context, kind, "", v, nil)
	case int:
		k = NewKey(d.Context, kind, "", int64(v), nil)
	case *Key:
		k = v
	default:
		return "", errors.New("Invalid key type")
	}

	k, err := nds.Put(d.Context, k, src)
	if err != nil {
		d.warn("%v, %v, %v, %#v", err, kind, k, src, d.Context)
		return "", err
	}
	return k.Encode(), nil
}

func (d *Datastore) PutMulti(kind string, srcs []interface{}) (keys []string, err error) {
	nkeys := len(srcs)
	_keys := make([]*Key, nkeys)
	log.Info(srcs)

	for i := 0; i < nkeys; i++ {
		_keys[i] = NewIncompleteKey(d.Context, kind, nil)
	}

	_keys, err = nds.PutMulti(d.Context, _keys, srcs)
	if err != nil {
		d.warn("%v", err, d.Context)
		return keys, err
	}

	keys = make([]string, nkeys)
	for i := 0; i < nkeys; i++ {
		keys[i] = _keys[i].Encode()
	}

	return keys, nil
}

func (d *Datastore) PutKeyMulti(kind string, keys []interface{}, srcs []interface{}) ([]*Key, error) {
	nkeys := len(srcs)
	_keys := make([]*Key, nkeys)

	for i := 0; i < nkeys; i++ {
		switch v := keys[i].(type) {
		case string:
			_keys[i] = NewKey(d.Context, kind, v, 0, nil)
		case int64:
			_keys[i] = NewKey(d.Context, kind, "", v, nil)
		case int:
			_keys[i] = NewKey(d.Context, kind, "", int64(v), nil)
		case *Key:
			_keys[i] = v
		default:
			return _keys, errors.New("Invalid key type")
		}
	}

	_keys, err := nds.PutMulti(d.Context, _keys, srcs)
	if err != nil {
		d.warn("%v", err, d.Context)
		return _keys, err
	}

	for i := 0; i < nkeys; i++ {
		keys[i] = _keys[i].Encode()
	}

	return _keys, nil
}

func (d *Datastore) Update(key string, src interface{}) (string, error) {
	d.warn("DEPRECATED. DOES NOTHING PUT DOES NOT.", d.Context)

	k, err := d.DecodeKey(key)
	if err != nil {
		return "", err
	}

	k, err = nds.Put(d.Context, k, src)
	if err != nil {
		d.warn("%v", err, d.Context)
		return "", err
	}
	return k.Encode(), nil
}

func (d *Datastore) Delete(key string) error {
	k, err := d.DecodeKey(key)
	if err != nil {
		return err
	}
	return nds.Delete(d.Context, k)
}

func (d *Datastore) DeleteMulti(keys []string) error {
	_keys := make([]*Key, 0)
	for _, key := range keys {
		k, err := d.DecodeKey(key)
		_keys = append(_keys, k)
		if err != nil {
			d.warn("%v", err, d.Context)
			return err
		}
	}
	return nds.DeleteMulti(d.Context, _keys)
}

func (d *Datastore) AllocateId(kind string) int64 {
	low, _, err := AllocateIDs(d.Context, kind, nil, 1)
	if err != nil {
		return 0
	}
	return low
}

func (d *Datastore) Query(kind string) *Query {
	return NewQuery(kind)
}

func (d *Datastore) RunInTransaction(f func(tc appengine.Context) error, opts *TransactionOptions) error {
	return nds.RunInTransaction(d.Context, f, opts)
}
