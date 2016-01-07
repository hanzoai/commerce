package multi

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	"crowdstart.com/models/mixin"
)

// Vals should be a slice of models
func multi(vals interface{}, fn func(mixin.Model) error) error {
	var wg sync.WaitGroup
	var err error

	switch reflect.TypeOf(vals).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(vals)

		for i := 0; i < s.Len(); i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Break if there is an error
				if err != nil {
					return
				}
				// Do something with model
				if model, ok := s.Index(i).Interface().(mixin.Model); ok {
					err = fn(model)
				} else {
					err = errors.New(fmt.Sprintf("Slice must contain models, not: %v", s.Index(i)))
				}
			}()
		}
	default:
		return errors.New(fmt.Sprintf("Must be called with slice of entities, not: %v", vals))
	}
	wg.Wait()
	return err
}

func Put(vals interface{}) error {
	return multi(vals, func(model mixin.Model) error {
		return model.Put()
	})
}

func Create(vals interface{}) error {
	return multi(vals, func(model mixin.Model) error {
		return model.Create()
	})
}

func Delete(vals interface{}) error {
	return multi(vals, func(model mixin.Model) error {
		return model.Delete()
	})
}

func Update(vals interface{}) error {
	return multi(vals, func(model mixin.Model) error {
		return model.Update()
	})
}

func MustPut(vals interface{}) {
	if err := Put(vals); err != nil {
		panic(err)
	}
}

func MustCreate(vals interface{}) {
	if err := Create(vals); err != nil {
		panic(err)
	}
}

func MustUpdate(vals interface{}) {
	if err := Update(vals); err != nil {
		panic(err)
	}
}

func MustDelete(vals interface{}) {
	if err := Delete(vals); err != nil {
		panic(err)
	}
}
