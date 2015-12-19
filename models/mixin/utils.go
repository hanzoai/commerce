package mixin

import (
	"reflect"

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
func zeroEntity(m *Model) Entity {
	typ := modelType(m)
	entity := reflect.New(typ)
	return entity.Interface().(Entity)
}

// Return a clone of current entity
func cloneEntity(m *Model) Entity {
	entity := zeroEntity(m)
	if err := structs.Copy(m.Entity, entity); err != nil {
		log.Warn("Unable to copy of model: %v", err, m.Db.Context)
	}
	return entity
}
