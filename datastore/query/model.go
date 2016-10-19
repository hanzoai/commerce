package query

import (
	"fmt"
	"reflect"
	"sync"

	"appengine"

	"crowdstart.com/datastore/iface"
	. "crowdstart.com/datastore/utils"
)

type Kind interface {
	Kind() string
}

// Subset of model API needed to initialize a model correctly.
type Model interface {
	SetContext(ctx interface{})
	SetEntity(entity interface{})
	SetKey(key interface{}) error
}

// Initialize model
func initModel(ctx appengine.Context, key iface.Key, value reflect.Value) {
	entity := value.Interface().(Kind)
	model := entity.(Model)
	model.SetContext(ctx)
	model.SetEntity(entity)
	model.SetKey(key)
}

// Fetches models and initializes them automatically. Dst must have type *[]*M,
// for some model type M.
func (q *Query) GetModels(dst interface{}) error {
	keys, err := q.aedsq.GetAll(q.ctx, dst)
	err = IgnoreFieldMismatch(err)

	if err != nil {
		return err
	}

	nkeys := len(keys)

	// Stop now if we found no models
	if nkeys == 0 {
		return nil
	}

	// Get slice
	slice := reflect.ValueOf(dst)
	if !isPtrSlice(slice) {
		return fmt.Errorf("Expected dst to be a pointer to a slice of models, got: %v", slice.Kind())
	}

	// De-pointer
	for slice.Kind() == reflect.Ptr {
		slice = reflect.Indirect(slice)
	}

	// Only a pointer to an entity will match the Entity interface
	if !isSliceOfPtr(slice) {
		return fmt.Errorf("Expected dst to be a pointer to a slice of models, got: %v", slice.Kind())
	}

	// Initialize all models in parallel
	var wg sync.WaitGroup

	for i := 0; i < nkeys; i++ {
		wg.Add(1)
		go func(ctx appengine.Context, key iface.Key, value reflect.Value) {
			initModel(ctx, key, value)
			wg.Done()
		}(q.ctx, keys[i], slice.Index(i))
	}

	// Wait to finish
	wg.Wait()

	return nil
}
