package datastore

import (
	"fmt"

	aeds "google.golang.org/appengine/datastore"

	"github.com/qedus/nds"
)

// Helper to create new incomplete key from string kind
func convertKeyOrKind(ds *Datastore, keyOrKind interface{}) *aeds.Key {
	switch v := keyOrKind.(type) {
	case string:
		return ds.NewIncompleteKey(v, nil)
	case Key:
		return convertKey(v)
	default:
		panic(fmt.Errorf("Invalid key or kind: %v", keyOrKind))
	}
}

// Puts entity using provided key or kind
func (d *Datastore) Put(keyOrKind interface{}, val interface{}) (*aeds.Key, error) {
	key := convertKeyOrKind(d, keyOrKind)
	if key, err := nds.Put(d.Context, key, val); err != nil {
		d.warn("Unable to put (%v, %#v): %v", key, val, err, d.Context)
		return nil, err
	} else {
		return key, err
	}
}

// Keys may be either either []datastore.Key or []*aeds.Key, vals expected in typical format
func (d *Datastore) PutMulti(keys interface{}, vals interface{}) ([]*aeds.Key, error) {
	return nds.PutMulti(d.Context, convertKeys(keys), vals)
}

// Gets an entity using datastore.Key or encoded Key
func (d *Datastore) MustPut(keyOrKind interface{}, value interface{}) *aeds.Key {
	if key, err := d.Put(keyOrKind, value); err != nil {
		panic(err)
	} else {
		return key
	}
}

// Gets an entity using datastore.Key or encoded Key
func (d *Datastore) MustPutMulti(keys interface{}, vals interface{}) []*aeds.Key {
	if keys, err := d.PutMulti(keys, vals); err != nil {
		panic(err)
	} else {
		return keys
	}
}
