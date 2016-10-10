package mixin

import (
	"crowdstart.com/datastore"
	"crowdstart.com/datastore/query"
)

// This is a simple Query helper for individual models. Allows you to query for
// a single entity or key only as a convenience on an individual model.
type ModelQuery struct {
	entity Entity
	dsq    datastore.Query
}

func (q *ModelQuery) All() datastore.Query {
	return q.dsq
}

func (q *ModelQuery) Ancestor(key datastore.Key) *ModelQuery {
	q.dsq = q.dsq.Ancestor(key)
	return q
}

func (q *ModelQuery) Limit(limit int) *ModelQuery {
	q.dsq = q.dsq.Limit(limit)
	return q
}

func (q *ModelQuery) Offset(offset int) *ModelQuery {
	q.dsq = q.dsq.Offset(offset)
	return q
}

func (q *ModelQuery) Order(order string) *ModelQuery {
	q.dsq = q.dsq.Order(order)
	return q
}

func (q *ModelQuery) Filter(filterStr string, value interface{}) *ModelQuery {
	q.dsq = q.dsq.Filter(filterStr, value)
	return q
}

// Get entity
func (q *ModelQuery) Get() (bool, error) {
	key, ok, err := q.dsq.First(q.entity)
	if ok {
		q.entity.SetKey(key)
		return true, nil
	}

	if err != nil {
		return false, err
	}

	return false, nil
}

// Get just key
func (q *ModelQuery) GetKey() (bool, error) {
	q.dsq = q.dsq.KeysOnly()
	return q.Get()
}

// Check if id exists
func (q *ModelQuery) IdExists(id string) (datastore.Key, bool, error) {
	return q.dsq.IdExists(id)
}

// Check if key exists
func (q *ModelQuery) KeyExists(key datastore.Key) (bool, error) {
	return q.dsq.KeyExists(key)
}

// Helper to set key if it's query returns one
func (q *ModelQuery) setAndForget(key datastore.Key, ok bool, err error) (bool, error) {
	if ok {
		q.entity.SetKey(key)
	}
	return ok, err
}

// Filter by key
func (q *ModelQuery) ByKey(key interface{}) (bool, error) {
	return q.setAndForget(q.dsq.ByKey(q.entity.Key(), q.entity))
}

// Filter By Id
func (q *ModelQuery) ById(id string) (bool, error) {
	return q.setAndForget(q.dsq.ById(id, q.entity))
}

// Return a query for this entity kind
func (m *Model) Query() *ModelQuery {
	q := new(ModelQuery)
	q.entity = m.Entity.(Entity)
	q.dsq = query.New(m.Context(), m.Kind())
	return q
}
