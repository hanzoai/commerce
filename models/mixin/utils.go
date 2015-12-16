package mixin

import (
	"reflect"

	"crowdstart.com/util/log"
)

// Get type of entity
func entityType(m *Model) reflect.Type {
	return reflect.ValueOf(m.Entity).Type()
}

// Return a zero'd entity of this type
func zeroEntity(m *Model) Entity {
	typ := entityType(m)
	return reflect.New(typ).Interface().(Entity)
}

// Return a clone of current entity
func cloneEntity(m *Model) Entity {
	entity := zeroEntity(m)
	if err := m.Db.Get(m.Key(), entity); err != nil {
		log.Warn("Unable to fetch copy of model from datastore: %v", err, m.Db.Context)
	}
	return entity
}
