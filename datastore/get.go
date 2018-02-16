package datastore

import (
	"errors"
	"reflect"

	"github.com/qedus/nds"

	"google.golang.org/appengine"
	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/util/log"
)

// Gets an entity using datastore.Key or encoded Key
func (d *Datastore) Get(key Key, value interface{}) error {
	aekey := key.(*aeds.Key)
	return d.ignoreFieldMismatch(nds.Get(d.Context, aekey, value))
}

// Gets an entity using datastore.Key or encoded Key
func (d *Datastore) GetById(id string, value interface{}) error {
	aekey := d.NewKeyFromId(id)
	return d.ignoreFieldMismatch(nds.Get(d.Context, aekey, value))
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
	aekeys := convertKeys(keys)
	nkeys := len(aekeys)

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

// Gets an entity using datastore.Key or encoded Key
func (d *Datastore) MustGet(key Key, value interface{}) {
	if err := d.Get(key, value); err != nil {
		panic(err)
	}
}

// Gets an entity using datastore.Key or encoded Key
func (d *Datastore) MustGetMulti(keys interface{}, vals interface{}) {
	if err := d.GetMulti(keys, vals); err != nil {
		panic(err)
	}
}
