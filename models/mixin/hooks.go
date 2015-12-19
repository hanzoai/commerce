package mixin

import "reflect"

type BeforeCreate interface {
	BeforeCreate() error
}

type BeforeUpdate interface {
	BeforeUpdate(Entity) error
}

type BeforeDelete interface {
	BeforeDelete() error
}

type AfterCreate interface {
	AfterCreate() error
}

type AfterUpdate interface {
	AfterUpdate(Entity) error
}

type AfterDelete interface {
	AfterDelete() error
}

// Try to get method off a model
func getMethod(name string, model *Model) (reflect.Method, bool) {
	typ := reflect.TypeOf(model.Entity)
	return typ.MethodByName(name)
}

// Call method returned by getMethod
func callMethod(method reflect.Method, previous Entity) error {
	args := []reflect.Value{
		reflect.ValueOf(previous),
	}

	ret := method.Func.Call(args)
	err, ok := ret[0].Interface().(error)
	if ok {
		return err
	}
	return nil
}
