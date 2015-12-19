package mixin

import (
	"reflect"

	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
	"crowdstart.com/util/structs"
)

// Get type of entity
func modelType(m *Model) reflect.Type {
	value := reflect.ValueOf(m.Entity)
	for value.Kind() == reflect.Ptr {
		value = reflect.Indirect(value)
	}
	return value.Type()
}

// Return a zero'd entity of this type
func (m *Model) Zero() Entity {
	typ := modelType(m)
	entity := reflect.New(typ)
	return entity.Interface().(Entity)
}

// Pulled out so it's easy to cache
func (m *Model) Clone() Entity {
	entity := m.Zero()
	if err := structs.Copy(m.Entity, entity); err != nil {
		log.Warn("Unable to copy of model: %v", err, m.Db.Context)
	}
	return entity
}

// Return Clone with only "public" JSON-serializable fields set
func (m *Model) CloneFromJSON() Entity {
	buf := json.EncodeBuffer(m.Entity)
	entity := m.Zero()
	json.DecodeBuffer(buf, entity)
	return entity
}

// Return slice suitable for use with GetAll
func (m *Model) Slice() interface{} {
	typ := reflect.TypeOf(m.Entity)
	slice := reflect.MakeSlice(reflect.SliceOf(typ), 0, 0)
	ptr := reflect.New(slice.Type())
	ptr.Elem().Set(slice)
	return ptr.Interface().(*[]Entity)
}

// Serialize entity to JSON
func (m *Model) JSON() []byte {
	return json.EncodeBytes(m.Entity)
}
