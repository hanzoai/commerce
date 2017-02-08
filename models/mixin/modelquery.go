package mixin

import (
	"reflect"

	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/util/log"
)

// Wrap Query so we don't need to pass in entity to First() and key is updated
// properly.
type Query struct {
	datastore.Query
	datastore *datastore.Datastore
	model     *Model
}

// Return a query for this entity kind
func (m *Model) Query() *Query {
	q := new(Query)
	query := datastore.NewQuery(m.Db, m.Entity.Kind())
	q.Query = query
	q.datastore = query.Datastore
	q.model = m
	return q
}

// Wrap datastore.Query
func (q *Query) Ancestor(key datastore.Key) *Query {
	q.Query = q.Query.Ancestor(key)
	return q
}

func (q *Query) Limit(limit int) *Query {
	q.Query = q.Query.Limit(limit)
	return q
}

func (q *Query) Offset(offset int) *Query {
	q.Query = q.Query.Offset(offset)
	return q
}

func (q *Query) Order(order string) *Query {
	q.Query = q.Query.Order(order)
	return q
}

func (q *Query) Filter(filterStr string, value interface{}) *Query {
	q.Query = q.Query.Filter(filterStr, value)
	return q
}

func (q *Query) KeysOnly() *Query {
	q.Query = q.Query.KeysOnly()
	return q
}

// Get Just first result
func (q *Query) First() (bool, error) {
	key, ok, err := q.Query.First(q.model.Entity)
	if ok {
		q.model.setKey(key)
	}
	return ok, err
}

// Get all models and return slice
func (q *Query) GetAll() (interface{}, error) {
	dst := q.model.Slice()
	keys, err := q.Query.GetAll(dst)

	// Get value of slice
	slice := getSlice(dst)

	// Initialize models
	for i := range keys {
		entity := slice.Index(i).Interface().(Entity)
		entity.Init(q.datastore)
		entity.SetKey(keys[i])
	}

	return slice.Interface(), err
}

// Get all models and return as slice of Entity
func (q *Query) GetEntities() ([]Entity, error) {
	dst := q.model.Slice()
	keys, err := q.Query.GetAll(dst)

	entities := make([]Entity, len(keys))

	// Get value of slice
	slice := getSlice(dst)

	for i := range keys {
		log.Debug("%v of (%v / %v)", i, slice.Len(), len(keys))
		entity := slice.Index(i).Interface().(Entity)
		entity.Init(q.datastore)
		entity.SetKey(keys[i])
		entities[i] = entity
	}

	return entities, err
}

// Load models into []Model or []*Model slice
func (q *Query) LoadAll(dst interface{}) ([]*aeds.Key, error) {
	keys, err := q.Query.GetAll(dst)
	if err != nil {
		return keys, err
	}

	// Stop now if we found no models
	if len(keys) == 0 {
		return keys, nil
	}

	// Get value of slice
	slice := getSlice(dst)

	// Check if models should be initialized, which only happens if []*Model is
	// passed as slice, only *Model matches Entity type.
	v := slice.Index(0)
	if v.Type().Kind() != reflect.Ptr {
		return keys, nil
	}

	// Initialize all entities
	for i := range keys {
		v = slice.Index(i)
		entity := v.Interface().(Entity)
		entity.Init(q.datastore)
		entity.SetKey(keys[i])
	}

	return keys, nil
}

// Get just keys
func (q *Query) GetKeys() ([]*aeds.Key, error) {
	return q.Query.KeysOnly().GetAll(nil)
}
