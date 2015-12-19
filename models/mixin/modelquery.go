package mixin

import (
	aeds "appengine/datastore"

	"crowdstart.com/datastore"
)

// Wrap Query so we don't need to pass in entity to First() and key is updated
// properly.
type Query struct {
	datastore.Query
	model *Model
}

// Return a query for this entity kind
func (m *Model) Query() *Query {
	q := new(Query)
	q.Query = datastore.NewQuery(m.Db, m.Entity.Kind())
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

func (q *Query) GetAll(dst interface{}) ([]*aeds.Key, error) {
	return q.Query.GetAll(dst)
}
