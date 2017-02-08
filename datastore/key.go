package datastore

import (
	"fmt"

	aeds "appengine/datastore"

	"hanzo.io/datastore/iface"
	"hanzo.io/datastore/key"
)

type Key iface.Key

var EncodeKey = key.Encode
var DecodeKey = key.Decode

func convertKey(key Key) *aeds.Key {
	if key == nil {
		return nil
	}
	return key.(*aeds.Key)
}

func convertKeys(keys interface{}) []*aeds.Key {
	switch v := keys.(type) {
	case []Key:
		n := len(v)
		aekeys := make([]*aeds.Key, n)
		for i := 0; i < n; i++ {
			aekeys[i] = v[i].(*aeds.Key)
		}
		return aekeys
	case []*aeds.Key:
		return v
	default:
		panic(fmt.Errorf("Invalid slice of keys: %v", keys))
	}
}

// Encode/decode hashid keys
func (d *Datastore) DecodeKey(encoded string) (*aeds.Key, error) {
	return DecodeKey(d.Context, encoded)
}

func (d *Datastore) EncodeKey(key Key) string {
	return EncodeKey(d.Context, key)
}

// Wrap appengine key funcs
func (d *Datastore) NewKey(kind, stringID string, intID int64, parent Key) *aeds.Key {
	return aeds.NewKey(d.Context, kind, stringID, intID, convertKey(parent))
}

func (d *Datastore) NewIncompleteKey(kind string, parent Key) *aeds.Key {
	return aeds.NewIncompleteKey(d.Context, kind, convertKey(parent))
}

// Create helpers
func (d *Datastore) NewKeyFromId(id string) *aeds.Key {
	return key.NewFromId(d.Context, id)
}

func (d *Datastore) NewKeyFromInt(kind string, id interface{}, parent Key) (*aeds.Key, error) {
	return key.NewFromInt(d.Context, kind, id, parent)
}

func (d *Datastore) NewKeyFromString(kind string, id string, parent Key) *aeds.Key {
	return d.NewKey(kind, id, 0, parent)
}

func (d *Datastore) AllocateID(kind string, parent Key) int64 {
	id, _ := d.AllocateIDs(kind, parent, 1)
	return id
}

func (d *Datastore) AllocateIDs(kind string, parent Key, n int) (int64, int64) {
	low, high, err := aeds.AllocateIDs(d.Context, kind, convertKey(parent), n)
	if err != nil {
		panic(fmt.Errorf("Unable to Allocate IDs: %v", err))
	}
	return low, high
}

func (d *Datastore) AllocateKey(kind string, parent Key) *aeds.Key {
	id := d.AllocateID(kind, parent)
	return d.NewKey(kind, "", id, parent)
}

// Datastore uses a key's ancestry to allocate unique integer IDs. If you
// allocate an ID with a nil parent you get an "orphaned" ID, i.e., an ID which
// does not use ancestry to determine uniqueness.  We have historically
// depended on this behavior for cheap, monotonically increasing order numbers
// (which are calculated from the key's integer id component).
func (d *Datastore) AllocateOrphanID(kind string) int64 {
	id, _ := d.AllocateIDs(kind, nil, 1)
	return id
}

func (d *Datastore) AllocateOrphanKey(kind string, parent Key) *aeds.Key {
	id := d.AllocateOrphanID(kind)
	return d.NewKey(kind, "", id, parent)
}
