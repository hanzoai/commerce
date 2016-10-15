package datastore

import (
	aeds "appengine/datastore"

	"github.com/qedus/nds"
)

// Puts entity, returning encoded key
func (d *Datastore) Put(key Key, val interface{}) (*aeds.Key, error) {
	if key, err := nds.Put(d.Context, convertKey(key), val); err != nil {
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
func (d *Datastore) MustPut(key Key, value interface{}) *aeds.Key {
	if key, err := d.Put(key, value); err != nil {
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
