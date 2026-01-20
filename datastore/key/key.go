package key

import (
	"context"
	"fmt"
	"strconv"

	aeds "google.golang.org/appengine/datastore"

	"github.com/qedus/nds"

	"github.com/hanzoai/commerce/datastore/iface"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/util/hashid"
)

type Key iface.Key

var (
	FromId = Decode
)

// Safely convert our Key to *aeds.Key
func convertKey(key Key) *aeds.Key {
	if key == nil {
		return nil
	}
	return key.(*aeds.Key)
}

// Convert or decode Key
func convertOrDecode(ctx context.Context, key interface{}) (*aeds.Key, error) {
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

// Return new key for given id type
func New(ctx context.Context, kind string, id interface{}, parent Key) *aeds.Key {
	pkey := convertKey(parent)
	switch v := id.(type) {
	case int64:
		return aeds.NewKey(ctx, kind, "", v, pkey)
	case int:
		return aeds.NewKey(ctx, kind, "", int64(v), pkey)
	case string:
		return aeds.NewKey(ctx, kind, v, 0, pkey)
	default:
		return aeds.NewIncompleteKey(ctx, kind, pkey)
	}
}

// Return key from hashid or appengine encoded strings
func NewFromId(ctx context.Context, id string) *aeds.Key {
	key, err := Decode(ctx, id)
	if err != nil {
		panic(err)
	}
	return key
}

// Return key from integer id
func NewFromInt(ctx context.Context, kind string, intid interface{}, parent Key) (*aeds.Key, error) {
	var id int64
	switch v := intid.(type) {
	case string:
		if parsed, err := strconv.ParseInt(v, 10, 64); err != nil {
			return nil, fmt.Errorf("Invalid integer for key: '%v'", intid)
		} else {
			id = parsed
		}
	case int64:
		id = v
	case int:
		id = int64(v)
	default:
		return nil, fmt.Errorf("Invalid integer for key: '%v'", intid)
	}

	return aeds.NewKey(ctx, kind, "", id, convertKey(parent)), nil
}

// Decode key encoded by aeds directly
func aedsDecode(ctx context.Context, key string) (*aeds.Key, error) {
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

// Encode key using hashid algorithm
func Encode(ctx context.Context, key Key) string {
	return hashid.EncodeKey(ctx, key)
}

// Decode key using hashid algorithm and falling back to base64 encoding
func Decode(ctx context.Context, encoded string) (*aeds.Key, error) {
	// Assume hashid
	key, err := hashid.DecodeKey(ctx, encoded)
	if err == nil {
		return key, nil
	}

	log.Debug("Failed to decode hashid, assuming base64 encoding: %v", err, ctx)

	// Fallback to aedsDecode
	return aedsDecode(ctx, encoded)
}

// Encode key with appengine's default base64 encoding
func Encode64(key Key) string {
	return key.Encode()
}

// Decode key encoded with appengine default base64 encoding
func Decode64(ctx context.Context, encoded string) (*aeds.Key, error) {
	return aedsDecode(ctx, encoded)
}

// Check if key exist in datastore
func Exists(ctx context.Context, key interface{}) (bool, error) {
	// Convert into Key
	k, err := convertOrDecode(ctx, key)
	if err != nil {
		return false, err
	}

	// Search for key in datastore
	err = nds.Get(ctx, k, nil)

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
