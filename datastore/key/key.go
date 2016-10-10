package key

import (
	"strconv"

	"appengine"
	aeds "appengine/datastore"

	"crowdstart.com/util/hashid"

	"crowdstart.com/datastore/iface"
)

type Key iface.Key

var (
	FromId = Decode
)

// Return new key
func New(ctx appengine.Context, kind string, key interface{}) *aeds.Key {
	switch v := key.(type) {
	case int64:
		return aeds.NewKey(ctx, kind, "", v, nil)
	case int:
		return aeds.NewKey(ctx, kind, "", int64(v), nil)
	case string:
		return aeds.NewKey(ctx, kind, v, 0, nil)
	default:
		return aeds.NewIncompleteKey(ctx, kind, nil)
	}
}

// Decode key encoded by aeds directly
func aedsDecode(ctx appengine.Context, key string) (*aeds.Key, error) {
	k, err := aeds.DecodeKey(key)

	// If unable to return key, bail out
	if err != nil {
		return nil, err
	}

	// Since key returned might have been created with a different app, we'll
	// recreate the key to ensure it has a valid AppID.
	k = aeds.NewKey(ctx, k.Kind(), k.StringID(), k.IntID(), nil)

	return k, nil
}

// Return Key from either string or int id.
func Encode(ctx appengine.Context, key Key) string {
	return hashid.EncodeKey(ctx, key)
}

// Decode key from string, supports hash ids and aeds encoded keys
func Decode(ctx appengine.Context, key string) (*aeds.Key, error) {
	// Assume hashid
	k, err := hashid.DecodeKey(ctx, key)
	if err == nil {
		return k, nil
	}

	// Fallback to aedsDecode
	return aedsDecode(ctx, key)
}

// Convert to Key from aeds.Key or encoded key
func convert(ctx appengine.Context, key interface{}) (*aeds.Key, error) {
	switch k := key.(type) {
	case *aeds.Key:
		return k, nil
	case iface.Key:
		return k.(*aeds.Key), nil
	case string:
		return Decode(ctx, k)
	default:
		return nil, aeds.ErrInvalidKey
	}
}

// Check for entity in datastore using key
func Exists(ctx appengine.Context, key interface{}) (bool, error) {
	// Convert into Key
	k, err := convert(ctx, key)
	if err != nil {
		return false, err
	}

	// Search for key in datastore
	_, err = aeds.NewQuery(k.Kind()).
		Filter("__key__", k).
		KeysOnly().
		GetAll(ctx, nil)

	// Not found
	if err == aeds.ErrNoSuchEntity {
		return false, nil
	}

	// Error querying for key
	if err != nil {
		return false, err
	}

	// Found it!
	return true, nil
}

// Return Key from int id (potentially a string int id).
func FromInt(ctx appengine.Context, kind string, intid interface{}) *aeds.Key {
	var id int64
	switch v := intid.(type) {
	case string:
		maybe, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			panic("Not a valid integer")
		}
		id = maybe
	case int64:
		id = v
	case int:
		id = int64(v)
	default:
		panic("Not a valid integer")
	}

	return aeds.NewKey(ctx, kind, "", id, nil)
}
