package multi

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	"appengine"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

// Vals should be a slice of models
func multi(vals interface{}, fn func(mixin.Entity) error) error {
	// Vals must be a slice
	if reflect.TypeOf(vals).Kind() != reflect.Slice {
		return errors.New(fmt.Sprintf("Must be called with slice of entities, not: %v", vals))
	}

	slice := reflect.ValueOf(vals)

	var wg sync.WaitGroup

	n := slice.Len()

	// Capture all errors
	errs := make(MultiError, n)
	haveErr := false

	// Loop over slice initializing entities
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			// Grab next entity off slice
			entity, ok := slice.Index(i).Interface().(mixin.Entity)
			if !ok {
				haveErr = true
				errs[i] = errors.New(fmt.Sprintf("Slice must contain entities, not: %v", slice.Index(i).Interface()))
			} else {
				if err := fn(entity); err != nil {
					haveErr = true
					errs[i] = err
				}
			}

			wg.Done()
		}(i)
	}

	// Wait to finish
	wg.Wait()

	if haveErr {
		return errs
	} else {
		return nil
	}
}

func Get(ctx appengine.Context, keys interface{}, vals interface{}) error {
	var wg sync.WaitGroup
	var valSlice reflect.Value

	db := datastore.New(ctx)

	// Keys must be a slice
	if reflect.TypeOf(keys).Kind() != reflect.Slice {
		return errors.New(fmt.Sprintf("Must be called with slice of keys, not: %v", keys))
	}

	keySlice := reflect.ValueOf(keys)
	nkeys := keySlice.Len()

	// Vals must be a slice
	typ := reflect.TypeOf(vals)
	switch typ.Kind() {
	case reflect.Ptr:
		valSlice = reflect.Indirect(reflect.ValueOf(vals))
	case reflect.Slice:
		valSlice = reflect.ValueOf(vals)
	default:
		return errors.New("Vals must be a slice or pointer to a slice")
	}

	// Get type of valSlice, values
	valSliceType := typ.Elem()
	valType := valSliceType.Elem()
	valType = reflect.Zero(valType).Type().Elem()

	// Auto allocate vals if length of valSlice is not set
	if valSlice.Len() == 0 {
		if !valSlice.CanAddr() {
			return errors.New("Destination must be addressable to auto-allocate entities")
		}

		// Create new valSlice of correct capacity and insert properly instantiated values
		zeroes := reflect.MakeSlice(valSliceType, nkeys, nkeys)

		// Append to vals valSlice, growing original valSlice to proper length
		valSlice.Set(reflect.AppendSlice(valSlice, zeroes))
	}

	// Capture all errors
	errs := make(MultiError, nkeys)
	haveErr := false

	// Loop over slice fetching entities
	for i := 0; i < nkeys; i++ {
		wg.Add(1)

		// Run method in gofunc
		go func(i int) {
			defer wg.Done()

			// Get key
			key := keySlice.Index(i).Interface()

			// Create new zero'd entity
			val := reflect.New(valType)
			entity := val.Interface().(mixin.Entity)

			// Initialize and try to fetch with key
			entity.Init(db)
			if err := entity.Get(key); err != nil {
				haveErr = true
				errs[i] = err
				return
			}

			// Set entity on val slice
			valSlice.Index(i).Set(val)
		}(i)
	}

	// Wait to finish
	wg.Wait()

	if haveErr {
		return errs
	} else {
		return nil
	}
}

func Put(vals interface{}) error {
	return multi(vals, func(entity mixin.Entity) error {
		return entity.Put()
	})
}

func Create(vals interface{}) error {
	return multi(vals, func(entity mixin.Entity) error {
		return entity.Create()
	})
}

func Delete(vals interface{}) error {
	return multi(vals, func(entity mixin.Entity) error {
		return entity.Delete()
	})
}

func Update(vals interface{}) error {
	return multi(vals, func(entity mixin.Entity) error {
		return entity.Update()
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
