package query

import (
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
	SetContext(ctx appengine.Context)
	SetEntity(entity Kind)
	SetKey(key iface.Key)
}

// De-pointer slice
func getSlice(iface interface{}) reflect.Value {
	// Get value of slice
	slice := reflect.ValueOf(iface)

	// De-pointer
	for slice.Kind() == reflect.Ptr {
		slice = reflect.Indirect(slice)
	}

	return slice
}

// Check if this is a slice of pointers
func sliceOfPtr(slice reflect.Value) bool {
	v := slice.Index(0)
	if v.Type().Kind() == reflect.Ptr {
		return true
	}

	return false
}

// Initialize model
func initModel(ctx appengine.Context, key iface.Key, value reflect.Value) {
	entity := value.Interface().(Kind)
	model := entity.(Model)
	model.SetContext(ctx)
	model.SetEntity(entity)
	model.SetKey(key)
}

// Load models into []Model or []*Model slice
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

	// Get value of slice
	slice := getSlice(dst)

	// Check if models should be initialized, only a slice of pointers will
	// matche Model interface.
	if !sliceOfPtr(slice) {
		return nil
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
