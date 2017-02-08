package mixin

import (
	"reflect"

	"hanzo.io/util/json"
	"hanzo.io/util/log"
	"hanzo.io/util/structs"
)

// Create a new zero'd entity of this type
func (m *Model) Zero() Entity {
	return reflect.New(Type(m.Entity)).Interface().(Entity)
}

// Create a clone of current entity
func (m *Model) Clone() Entity {
	entity := m.Zero()
	if err := structs.Copy(m.Entity, entity); err != nil {
		log.Warn("Unable to copy of model: %v", err, m.Db.Context)
	}
	return entity
}

// Create a clone of currenty entity using only JSON-serializable data
func (m *Model) CloneFromJSON() Entity {
	buf := json.EncodeBuffer(m.Entity)
	entity := m.Zero()
	json.DecodeBuffer(buf, entity)
	return entity
}

// Create a slice of entity type suitable for use with datastore.GetAll, etc.
func (m *Model) Slice() interface{} {
	// *Model, since this is a pointer method
	typ := reflect.TypeOf(m.Entity)

	// Create slice of *Model
	slice := reflect.MakeSlice(reflect.SliceOf(typ), 0, 0)

	// Get pointer to slice (necessary cuz Go sucks)
	ptr := reflect.New(slice.Type())
	ptr.Elem().Set(slice)
	return ptr.Interface()
}

// Create a slice of de-pointered entity type suitable for use with datastore.GetMulti, etc.
func (m *Model) ValueSlice(len int) interface{} {
	// *Model, since this is a pointer method
	val := reflect.ValueOf(m.Entity)

	for val.Kind() == reflect.Ptr {
		val = reflect.Indirect(val)
	}

	typ := val.Type()

	// Create slice of *Model
	slice := reflect.MakeSlice(reflect.SliceOf(typ), len, len)

	return slice.Interface()
}

// Serialize entity to JSON
func (m *Model) JSON() []byte {
	return json.EncodeBytes(m.Entity)
}
