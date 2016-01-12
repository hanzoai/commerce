package mixin

import (
	"reflect"

	aeds "appengine/datastore"

	"crowdstart.com/datastore"
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

func (q *Query) First() (bool, error) {
	key, ok, err := q.Query.First(q.model.Entity)
	if ok {
		q.model.setKey(key)
	}
	return ok, err
}

// Dst expected to be *[]*Model
func (q *Query) GetAll(dst interface{}) ([]*aeds.Key, error) {
	keys, err := q.Query.GetAll(dst)
	if err != nil {
		return keys, err
	}

	// Get value of slice
	value := reflect.ValueOf(dst)

	// De-pointer
	for value.Kind() == reflect.Ptr {
		value = reflect.Indirect(value)
	}

	// Initialize all entities
	for i := range keys {
		entity := value.Index(i).Interface().(Entity)
		entity.Init(q.datastore)
		entity.SetKey(keys[i])
	}

	return keys, nil
}

// Get just keys
func (q *Query) GetKeys() ([]*aeds.Key, error) {
	return q.Query.KeysOnly().GetAll(nil)
}

func (q *Query) GetEntities() ([]Entity, error) {
	islice := q.model.Slice()
	keys, err := q.Query.GetAll(islice)

	value := reflect.ValueOf(islice)
	slice := make([]Entity, len(keys))

	for i := range keys {
		entity := value.Index(i).Interface().(Entity)
		entity.Init(q.datastore)
		entity.SetKey(keys[i])
		slice[i] = entity
	}

	return slice, err
}
