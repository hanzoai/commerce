package datastore

import (
	"errors"
	"reflect"
	"strconv"

	"appengine"

	aeds "appengine/datastore"

	"github.com/gin-gonic/gin"
	"github.com/qedus/nds"

	"crowdstart.io/config"
	"crowdstart.io/util/log"
)

// Alias Done error
var (
	Done       = aeds.Done
	InvalidKey = errors.New("Invalid key")
)

type Datastore struct {
	Context             appengine.Context
	IgnoreFieldMismatch bool
	Warn                bool
}

func New(ctx interface{}) (d *Datastore) {
	switch ctx := ctx.(type) {
	case appengine.Context:
		d = &Datastore{ctx, config.DatastoreWarn, true}
	case *gin.Context:
		c := ctx.MustGet("appengine").(appengine.Context)
		d = &Datastore{c, config.DatastoreWarn, true}
	}
	return d
}

// Private method for logging selectively
func (d *Datastore) warn(fmtOrError interface{}, args ...interface{}) {
	if d.Warn {
		log.Warn(fmtOrError, args...)
	}
}

// Helper to ignore tedious field mismatch errors (but warn appropriately
// during development)
func (d *Datastore) SkipFieldMismatch(err error) error {
	// Ignore nil error or `IgnoreFieldMismatch` is disabled
	if err == nil || !d.IgnoreFieldMismatch {
		return nil
	}

	if _, ok := err.(*aeds.ErrFieldMismatch); ok {
		// Ignore any field mismatch errors.
		d.warn("Ignoring, %v", err, d.Context)
		return nil
	}

	return err
}

// Return Key from either string or int id.
func (d *Datastore) KeyFromId(kind string, id interface{}) Key {
	switch v := id.(type) {
	case string:
		return aeds.NewKey(d.Context, kind, v, 0, nil)
	case int64:
		return aeds.NewKey(d.Context, kind, "", v, nil)
	case int:
		return aeds.NewKey(d.Context, kind, "", int64(v), nil)
	default:
		d.warn("EncodeId was passed an invalid type %v", v)
		return aeds.NewIncompleteKey(d.Context, kind, nil)
	}
}

// Return Key from int id (potentially a string int id).
func (d *Datastore) KeyFromInt(kind string, id interface{}) Key {
	var _id int64
	switch v := id.(type) {
	case string:
		maybeId, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			d.warn("EncodeId was passed an string that could not be parsed to int64 %v", v)
			return aeds.NewIncompleteKey(d.Context, kind, nil)
		}
		_id = maybeId
	case int64:
		_id = v
	case int:
		_id = int64(v)
	default:
		d.warn("EncodeId was passed an invalid type %v", v)
		return aeds.NewIncompleteKey(d.Context, kind, nil)
	}

	return aeds.NewKey(d.Context, kind, "", _id, nil)
}

// Return an encoded key from an id representation
func (d *Datastore) EncodeId(kind string, id interface{}) string {
	key := d.KeyFromInt(kind, id)

	// If we KeyFromInt returns an incomplete key, return empty string.
	if key.IntID() == 0 {
		return ""
	}

	return key.Encode()
}

// Wrap new key funcs
func (d *Datastore) NewIncompleteKey(kind string, parent Key) *aeds.Key {
	if p, ok := parent.(*aeds.Key); ok {
		return aeds.NewIncompleteKey(d.Context, kind, p)
	} else {
		return aeds.NewIncompleteKey(d.Context, kind, nil)
	}
}

func (d *Datastore) NewKey(kind, stringID string, intID int64, parent Key) *aeds.Key {
	if p, ok := parent.(*aeds.Key); ok {
		return aeds.NewKey(d.Context, kind, stringID, intID, p)
	} else {
		return aeds.NewKey(d.Context, kind, stringID, intID, nil)
	}
}

func (d *Datastore) DecodeKey(encodedKey string) (*aeds.Key, error) {
	_key, err := aeds.DecodeKey(encodedKey)

	// If unable to return key, bail out
	if err != nil {
		d.warn("Unable to decode key: %v", encodedKey)
		return _key, err
	}

	// Since key returned might have been created with a different app, we'll
	// recreate the key to ensure it has a valid AppID.
	key := aeds.NewKey(d.Context, _key.Kind(), _key.StringID(), _key.IntID(), nil)

	return key, err
}

// Helper func to get key for `datastore.Get/datastore.GetMulti`
func (d *Datastore) keyOrEncodedKey(key interface{}) (_key *aeds.Key, err error) {
	// Get datastore.Key if necessary
	switch v := key.(type) {
	case *aeds.Key:
		return v, nil
	case string:
		return d.DecodeKey(v)
	case reflect.Value:
		return d.keyOrEncodedKey(v.Interface())
	default:
		return _key, InvalidKey
	}
}

// Return either an incomplete key if passed just the kind, or key
func (d *Datastore) keyOrKind(keyOrKind interface{}) (_key *aeds.Key, err error) {
	// Try to construct a datastore key from whatever we were given as a key
	switch v := keyOrKind.(type) {
	case string:
		return aeds.NewIncompleteKey(d.Context, v, nil), nil
	case *aeds.Key:
		return v, nil
	default:
		return _key, InvalidKey
	}
}

// Helper func to get key for `datastore.GetKind/datastore.GetKindMulti`
func (d *Datastore) keyOrKindKey(kind string, key interface{}) (_key *aeds.Key, err error) {
	// Try to construct a datastore key from whatever we were given as a key
	switch v := key.(type) {
	case string:
		_key = aeds.NewKey(d.Context, kind, v, 0, nil)
	case int64:
		_key = aeds.NewKey(d.Context, kind, "", v, nil)
	case int:
		_key = aeds.NewKey(d.Context, kind, "", int64(v), nil)
	case *aeds.Key:
		_key = v
	case nil:
		_key = aeds.NewIncompleteKey(d.Context, kind, nil)
	case reflect.Value:
		return d.keyOrKindKey(kind, v.Interface())
	default:
		return _key, InvalidKey
	}

	return _key, nil
}

// Gets an entity using datastore.Key or encoded Key
func (d *Datastore) Get(key interface{}, value interface{}) error {
	_key, err := d.keyOrEncodedKey(key)

	// Invalid key, bail out.
	if err != nil {
		d.warn("Invalid key: unable to get %v: %v", key, err)
		return err
	}

	// Try to retrieve entity using nds, which transparently uses memcache if possible
	return d.SkipFieldMismatch(nds.Get(d.Context, _key, value))
}

// Gets an entity by literal datastore key of string type
func (d *Datastore) GetKind(kind string, key interface{}, value interface{}) error {
	_key, err := d.keyOrKindKey(kind, key)

	// Invalid key, bail out.
	if err != nil {
		d.warn("Invalid key: unable to get (%v, %v): %v", kind, key, err)
		return err
	}

	// Try to retrieve entity using nds, which transparently uses memcache if possible
	return d.SkipFieldMismatch(nds.Get(d.Context, _key, value))
}

// Same as Get, but works for multiple key/vals, keys can be slice of any type
// accepted by Get
func (d *Datastore) GetMulti(keys interface{}, vals interface{}) error {
	var slice reflect.Value

	switch reflect.TypeOf(keys).Kind() {
	case reflect.Slice:
		slice = reflect.ValueOf(keys)
	default:
		return errors.New("Keys must be a slice.")
	}

	nkeys := slice.Len()
	_keys := make([]*aeds.Key, nkeys)

	for i := 0; i < nkeys; i++ {
		key, err := d.keyOrEncodedKey(slice.Index(i))
		if err != nil {
			d.warn("Invalid key: unable to get %v: %v", key, err)
			return err
		}
		_keys[i] = key
	}

	return d.SkipFieldMismatch(nds.GetMulti(d.Context, _keys, vals))
}

// Same as GetKind, but works for multiple key/vals, keys can be slice of any
// type accepted by GetKind
func (d *Datastore) GetKindMulti(kind string, keys interface{}, vals interface{}) error {
	var slice reflect.Value

	switch reflect.TypeOf(keys).Kind() {
	case reflect.Slice:
		slice = reflect.ValueOf(keys)
	default:
		return errors.New("Keys must be a slice.")
	}

	nkeys := slice.Len()
	_keys := make([]*aeds.Key, nkeys)

	for i := 0; i < nkeys; i++ {
		key, err := d.keyOrKindKey(kind, slice.Index(i))
		if err != nil {
			d.warn("Invalid key: unable to get %v: %v", key, err)
			return err
		}
		_keys[i] = key
	}

	return d.SkipFieldMismatch(nds.GetMulti(d.Context, _keys, vals))
}

// Puts entity, returning encoded key
func (d *Datastore) Put(keyOrKind interface{}, src interface{}) (*aeds.Key, error) {
	key, err := d.keyOrKind(keyOrKind)
	if err != nil {
		return key, err
	}

	key, err = nds.Put(d.Context, key, src)
	if err != nil {
		d.warn("Unable to put (%v, %#v): %v", keyOrKind, src, err, d.Context)
		return key, err
	}
	return key, nil
}

func (d *Datastore) PutKind(kind string, key interface{}, src interface{}) (*aeds.Key, error) {
	_key, err := d.keyOrKindKey(kind, key)

	// Invalid key, bail out.
	if err != nil {
		d.warn("Invalid key: unable to put (%v, %v, %#v): %v", kind, key, src, err)
		return _key, err
	}

	_key, err = nds.Put(d.Context, _key, src)
	if err != nil {
		d.warn("%v, %v, %v, %#v", err, kind, _key, src, d.Context)
		return _key, err
	}
	return _key, nil
}

func (d *Datastore) PutMulti(kind string, srcs []interface{}) (keys []*aeds.Key, err error) {
	nkeys := len(srcs)
	_keys := make([]*aeds.Key, nkeys)
	log.Info(srcs)

	for i := 0; i < nkeys; i++ {
		_keys[i] = aeds.NewIncompleteKey(d.Context, kind, nil)
	}

	_keys, err = nds.PutMulti(d.Context, _keys, srcs)
	if err != nil {
		d.warn("%v", err, d.Context)
		return keys, err
	}

	return _keys, nil
}

func (d *Datastore) PutKindMulti(kind string, keys []interface{}, srcs []interface{}) ([]*aeds.Key, error) {
	nkeys := len(srcs)
	_keys := make([]*aeds.Key, nkeys)

	for i := 0; i < nkeys; i++ {
		key := keys[i]
		if _key, err := d.keyOrKindKey(kind, key); err != nil {
			d.warn("Invalid key: unable to put (%v, %v, %v): %v", kind, key, srcs[i], err)
			return _keys, err
		} else {
			_keys[i] = _key
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

func (d *Datastore) Delete(key interface{}) error {
	_key, err := d.keyOrEncodedKey(key)
	if err != nil {
		d.warn("Invalid key: unable to get %v: %v", key, err)
		return err
	}

	return nds.Delete(d.Context, _key)
}

func (d *Datastore) DeleteMulti(keys []string) error {
	_keys := make([]*aeds.Key, 0)
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
	low, _, err := aeds.AllocateIDs(d.Context, kind, nil, 1)
	if err != nil {
		d.warn("Unable to allocate id for '%v': %v", kind, err)
		return 0
	}
	return low
}

func (d *Datastore) AllocateIntKey(kind string) Key {
	id := d.AllocateId(kind)
	return aeds.NewKey(d.Context, kind, "", id, nil)
}

func (d *Datastore) Query(kind string) *aeds.Query {
	return aeds.NewQuery(kind)
}

func (d *Datastore) Query2(kind string) Query {
	return NewQuery(kind, d)
}

func (d *Datastore) RunInTransaction(f func(tc appengine.Context) error, opts *aeds.TransactionOptions) error {
	return nds.RunInTransaction(d.Context, f, opts)
}

// Helper to ignore tedious field mismatch errors (but warn appropriately
// during development)
func IgnoreFieldMismatch(err error) error {
	if err == nil {
		// Ignore nil error
		return nil
	}

	if _, ok := err.(*aeds.ErrFieldMismatch); ok {
		// Ignore any field mismatch errors, but warn user (at least during development)
		log.Warn("Ignoring, %v", err)
		return nil
	}

	// Any other errors we damn well need to know about!
	return err
}
