package datastore

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"appengine"

	aeds "appengine/datastore"

	"github.com/gin-gonic/gin"
	"github.com/qedus/nds"

	"crowdstart.com/config"
	"crowdstart.com/util/log"

	"crowdstart.com/datastore/query"
	"crowdstart.com/datastore/utils"
)

var (
	// Alias appengine types
	Done                 = aeds.Done
	ErrNoSuchEntity      = aeds.ErrNoSuchEntity
	ErrInvalidEntityType = aeds.ErrInvalidEntityType
	ErrInvalidKey        = aeds.ErrInvalidKey

	// Alias utils
	IgnoreFieldMismatch = utils.IgnoreFieldMismatch

	// Custom Errors
	InvalidKey  = ErrInvalidKey
	KeyNotFound = ErrNoSuchEntity
)

type Datastore struct {
	Context             appengine.Context
	IgnoreFieldMismatch bool
	Warn                bool
}

func New(ctx interface{}) *Datastore {
	d := new(Datastore)
	d.IgnoreFieldMismatch = true
	d.Warn = config.DatastoreWarn
	d.SetContext(ctx)
	return d
}

// Private method for logging selectively
func (d *Datastore) warn(fmtOrError interface{}, args ...interface{}) {
	if d.Warn {
		log.Warn(fmtOrError, args...)
	}
}

// Helper to ignore tedious field mismatch errors
func (d *Datastore) ignoreFieldMismatch(err error) error {
	if d.IgnoreFieldMismatch {
		return IgnoreFieldMismatch(err)
	}
	return err
}

// Set context for datastore
func (d *Datastore) SetContext(ctx interface{}) {
	switch ctx := ctx.(type) {
	case appengine.Context:
		d.Context = ctx
	case *gin.Context:
		d.Context = ctx.MustGet("appengine").(appengine.Context)
	}
}

// Set context for datastore
func (d *Datastore) SetNamespace(ns string) {
	if ctx, err := appengine.Namespace(d.Context, ns); err != nil {
		log.Error("Unable to set namespace for datastore: %v", err, d.Context)
	} else {
		d.Context = ctx
	}
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

func (d *Datastore) DecodeKey(encoded string) (*aeds.Key, error) {
	return DecodeKey(d.Context, encoded)
}

// Helper func to get key for `datastore.Get/datastore.GetMulti`
func (d *Datastore) keyOrEncodedKey(key interface{}) (_key *aeds.Key, err error) {
	// Get datastore.Key if necessary
	switch v := key.(type) {
	case *aeds.Key:
		return v, nil
	case Key:
		return v.(*aeds.Key), nil
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
	return d.ignoreFieldMismatch(nds.Get(d.Context, _key, value))
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
	return d.ignoreFieldMismatch(nds.Get(d.Context, _key, value))
}

// Same as Get, but works for multiple key/vals, keys can be slice of any type
// accepted by GetMulti as well as *[]*Model, which will automatically
// allocated if necessary.
func (d *Datastore) GetMulti(keys interface{}, vals interface{}) error {
	var slice reflect.Value

	// Check keys type
	switch reflect.TypeOf(keys).Kind() {
	case reflect.Slice:
		slice = reflect.ValueOf(keys)
	default:
		return errors.New("Keys must be a slice")
	}

	// Convert keys to appropriate type
	nkeys := slice.Len()
	aekeys := make([]*aeds.Key, nkeys)
	for i := 0; i < nkeys; i++ {
		key, err := d.keyOrEncodedKey(slice.Index(i))
		if err != nil {
			d.warn("Invalid key: unable to get %v: %v", key, err)
			return err
		}
		aekeys[i] = key
	}

	// Check type of vals
	typ := reflect.TypeOf(vals)
	switch typ.Kind() {
	case reflect.Ptr:
		slice = reflect.Indirect(reflect.ValueOf(vals))
	case reflect.Slice:
		slice = reflect.ValueOf(vals)
	default:
		return errors.New("Vals must be a slice or pointer to a slice")
	}

	// Auto allocate vals if length of slice is not set
	if slice.Len() == 0 {
		log.Warn("Got slice of len 0: %v", slice)
		if !slice.CanAddr() {
			return errors.New("Destination must be addressable to auto-allocate entities")
		}

		// Get type of slice, values
		sliceType := typ.Elem()
		valType := sliceType.Elem()
		valType = reflect.Zero(valType).Type().Elem()

		// Create new slice of correct capacity and insert properly instantiated values
		zeroes := reflect.MakeSlice(sliceType, nkeys, nkeys)
		for i := 0; i < nkeys; i++ {
			zero := reflect.New(valType)
			zeroes.Index(i).Set(zero)
		}

		// Append to vals slice, growing original slice to proper length
		slice.Set(reflect.AppendSlice(slice, zeroes))
	}

	// Fetch entities from datastore
	err := d.ignoreFieldMismatch(nds.GetMulti(d.Context, aekeys, slice.Interface()))
	if err != nil {
		if me, ok := err.(appengine.MultiError); ok {
			for _, merr := range me {
				log.Warn(merr, d.Context)
			}
		}
	}
	return err
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

	return d.ignoreFieldMismatch(nds.GetMulti(d.Context, _keys, vals))
}

// Puts entity, returning encoded key
func (d *Datastore) Put(keyOrKind interface{}, val interface{}) (*aeds.Key, error) {
	key, err := d.keyOrKind(keyOrKind)
	if err != nil {
		return key, err
	}

	key, err = nds.Put(d.Context, key, val)
	if err != nil {
		d.warn("Unable to put (%v, %#v): %v", keyOrKind, val, err, d.Context)
		return key, err
	}
	return key, nil
}

func (d *Datastore) PutKind(kind string, key interface{}, val interface{}) (*aeds.Key, error) {
	_key, err := d.keyOrKindKey(kind, key)

	// Invalid key, bail out.
	if err != nil {
		d.warn("Invalid key: unable to put (%v, %v, %#v): %v", kind, key, val, err)
		return _key, err
	}

	_key, err = nds.Put(d.Context, _key, val)
	if err != nil {
		d.warn("%v, %v, %v, %#v", err, kind, _key, val, d.Context)
		return _key, err
	}
	return _key, nil
}

// Keys may be either either []datastore.Key or []*aeds.Key, vals expected in typical format
func (d *Datastore) PutMulti(keys interface{}, vals interface{}) ([]*aeds.Key, error) {
	var _keys []*aeds.Key

	switch v := keys.(type) {
	case []Key:
		n := len(v)
		_keys = make([]*aeds.Key, n)

		for i := 0; i < n; i++ {
			_keys[i] = v[i].(*aeds.Key)
		}
	case []*aeds.Key:
		_keys = v
	default:
		return _keys, errors.New(fmt.Sprintf("Invalid slice of keys: %v", keys))
	}

	return nds.PutMulti(d.Context, _keys, vals)
}

func (d *Datastore) MustPutMulti(keys interface{}, vals interface{}) ([]*aeds.Key, error) {
	_keys, err := d.PutMulti(keys, vals)
	if err != nil {
		panic(err)
	}
	return _keys, err
}

func (d *Datastore) PutKindMulti(kind string, keys []interface{}, vals []interface{}) ([]*aeds.Key, error) {
	nkeys := len(vals)
	_keys := make([]*aeds.Key, nkeys)

	for i := 0; i < nkeys; i++ {
		key := keys[i]
		if _key, err := d.keyOrKindKey(kind, key); err != nil {
			d.warn("Invalid key: unable to put (%v, %v, %v): %v", kind, key, vals[i], err)
			return _keys, err
		} else {
			_keys[i] = _key
		}
	}

	_keys, err := nds.PutMulti(d.Context, _keys, vals)
	if err != nil {
		d.warn("%v", err, d.Context)
		return _keys, err
	}

	for i := 0; i < nkeys; i++ {
		keys[i] = _keys[i].Encode()
	}

	return _keys, nil
}

func (d *Datastore) Update(key string, val interface{}) (string, error) {
	d.warn("DEPRECATED. DOES NOTHING PUT DOES NOT.", d.Context)

	k, err := d.DecodeKey(key)
	if err != nil {
		return "", err
	}

	k, err = nds.Put(d.Context, k, val)
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

func (d *Datastore) DeleteMulti(keys []*aeds.Key) error {
	return nds.DeleteMulti(d.Context, keys)
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

// Return a *datastore.Query
func (d *Datastore) Query(kind string) Query {
	return query.New(d.Context, kind)
}

func (d *Datastore) RunInTransaction(fn func(db *Datastore) error, opts ...*TransactionOptions) error {
	return RunInTransaction(d.Context, fn, opts...)
}

func (d *Datastore) DecodeCursor(cursor string) (aeds.Cursor, error) {
	return aeds.DecodeCursor(cursor)
}

type TransactionOptions aeds.TransactionOptions

func RunInTransaction(ctx appengine.Context, fn func(db *Datastore) error, opts ...*TransactionOptions) error {
	aeopts := new(aeds.TransactionOptions)

	if len(opts) > 0 {
		aeopts.XG = opts[0].XG
		aeopts.Attempts = opts[0].Attempts
	}

	return nds.RunInTransaction(ctx, func(ctx appengine.Context) error {
		return fn(New(ctx))
	}, aeopts)
}
