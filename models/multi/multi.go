package multi

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/util/log"
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
	errd := false

	// Loop over slice initializing entities
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// Grab next entity off slice
			val := slice.Index(i)

			// Ensure valid pointer to model
			if val.Kind() != reflect.Ptr {
				errd = true
				errs[i] = errors.New(fmt.Sprintf("Slice must contain pointers to models, not %v", val))
				return
			}

			// Ensure not nil pointer to model
			if val.IsNil() {
				errd = true
				errs[i] = errors.New(fmt.Sprintf("Slice must contain initialized models, not %v", val))
				return
			}

			// Assert entity is valid
			entity, ok := slice.Index(i).Interface().(mixin.Entity)
			if !ok {
				errd = true
				errs[i] = errors.New(fmt.Sprintf("Slice must contain entities, not %v", slice.Index(i).Interface()))
				return
			}

			// Run operation on entity
			if err := fn(entity); err != nil {
				errd = true
				errs[i] = err
			}
		}(i)
	}

	// Wait to finish
	wg.Wait()

	if errd {
		return errs
	} else {
		return nil
	}
}

func Get(db *datastore.Datastore, keys interface{}, vals interface{}) error {
	keySlice := reflect.ValueOf(keys)
	var valSlice reflect.Value

	// Keys must be a slice
	if reflect.TypeOf(keys).Kind() != reflect.Slice {
		return fmt.Errorf("Keys must be a slice of keys, not: %v", keys)
	}

	// Vals must be a slice
	typ := reflect.TypeOf(vals)
	switch typ.Kind() {
	case reflect.Ptr:
		valSlice = reflect.Indirect(reflect.ValueOf(vals))
	case reflect.Slice:
		valSlice = reflect.ValueOf(vals)
	default:
		return fmt.Errorf("Vals must be a slice or pointer to a slice, not: %v", vals)
	}

	// Get number of entities we're fetching
	nkeys := keySlice.Len()

	// Get type of valSlice, values
	valType := typ.Elem()
	valType = reflect.Zero(valType).Type()

	// Auto allocate vals if length of valSlice is not set
	if valSlice.Len() == 0 {
		if !valSlice.CanAddr() {
			return errors.New("Destination must be addressable to auto-allocate entities")
		}

		// Create new valSlice of correct capacity and insert properly instantiated values
		zeroes := reflect.MakeSlice(typ, nkeys, nkeys)

		// Append to vals valSlice, growing original valSlice to proper length
		valSlice.Set(reflect.AppendSlice(valSlice, zeroes))
	}

	// Create new zero'd entity
	log.Debug("valtype: %v", valType)

	// Capture all errors
	errs := make(MultiError, nkeys)
	errd := false

	var wg sync.WaitGroup

	// Loop over slice fetching entities
	for i := 0; i < nkeys; i++ {
		wg.Add(1)

		// Run method in gofunc
		go func(i int) {
			defer wg.Done()

			// Get key
			key := keySlice.Index(i).Interface()

			val := reflect.New(valType)
			entity := val.Interface().(mixin.Entity)

			// Initialize and try to fetch with key
			entity.Init(db)
			if err := entity.Get(key); err != nil {
				errd = true
				errs[i] = err
				return
			}

			// Set entity on val slice
			valSlice.Index(i).Set(val)
		}(i)
	}

	// Wait to finish
	wg.Wait()

	if errd {
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
