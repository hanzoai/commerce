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
	Datastore *datastore.Datastore
	Model     *Model
}

// Return a query for this entity kind
func (m *Model) Query() *Query {
	q := new(Query)
	query := datastore.NewQuery(m.Db, m.Entity.Kind())
	q.Query = query
	q.Datastore = query.Datastore
	q.Model = m
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
	key, ok, err := q.Query.First(q.Model.Entity)
	if err != nil {
		return false, err
	}

	if !ok {
		return false, datastore.ErrNoSuchEntity
	}

	q.Model.setKey(key)

	return true, nil
}

// Dst expected to be *[]*Model
func (q *Query) GetAll(dst interface{}) ([]*aeds.Key, error) {
	keys, err := q.Query.GetAll(dst)
	if err != nil {
		return keys, err
	}

	if dst == nil {
		return keys, nil
	}

	// Get value of slice
	slice := reflect.ValueOf(dst)

	// De-pointer
	for slice.Kind() == reflect.Ptr {
		slice = reflect.Indirect(slice)
	}

	// Initialize all entities (if pointer type)
	for i := range keys {
		v := slice.Index(i)
		if v.Type().Kind() == reflect.Ptr {
			entity := v.Interface().(Entity)
			entity.Init(q.Datastore)
			entity.SetKey(keys[i])
		}

		// NOTE: Or we could do something like this instead, to support both
		// entity values and pointers:
		// v := slice.Index(i)

		// var entity Entity
		// if v.Type().Kind() == reflect.Ptr {
		// 	// We have a pointer, this is a valid entity
		// 	entity = v.Interface().(Entity)
		// } else {
		// 	// If we do not have a pointer we need to get one to this entity
		// 	ptr := reflect.New(reflect.TypeOf(v))
		// 	tmp := ptr.Elem()
		// 	tmp.Set(v)
		// 	entity = tmp.Interface().(Entity)
		// }

		// entity.Init(q.datastore)
		// entity.SetKey(keys[i])
	}

	return keys, nil
}

// Get just keys
func (q *Query) GetKeys() ([]*aeds.Key, error) {
	return q.Query.KeysOnly().GetAll(nil)
}

func (q *Query) GetEntities() ([]Entity, error) {
	islice := q.Model.Slice()
	keys, err := q.Query.GetAll(islice)

	value := reflect.ValueOf(islice)
	// De-pointer
	for value.Kind() == reflect.Ptr {
		value = reflect.Indirect(value)
	}

	slice := make([]Entity, len(keys))

	for i := range keys {
		entity := value.Index(i).Interface().(Entity)
		entity.Init(q.Datastore)
		entity.SetKey(keys[i])
		slice[i] = entity
	}

	return slice, err
}
