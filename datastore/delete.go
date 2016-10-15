package datastore

import "github.com/qedus/nds"

// Delete an entity
func (d *Datastore) Delete(key Key) error {
	return nds.Delete(d.Context, convertKey(key))
}

func (d *Datastore) DeleteMulti(keys interface{}) error {
	return nds.DeleteMulti(d.Context, convertKeys(keys))
}

// Gets an entity using datastore.Key or encoded Key
func (d *Datastore) MustDelete(key Key) {
	if err := d.Delete(key); err != nil {
		panic(err)
	}
}

// Gets an entity using datastore.Key or encoded Key
func (d *Datastore) MustDeleteMulti(keys interface{}) {
	if err := d.DeleteMulti(keys); err != nil {
		panic(err)
	}
}
