package datastore

import (
	"appengine"
	aeds "appengine/datastore"
)

type Query interface {
	Ancestor(ancestor Key) *DatastoreQuery
	Count() (int, error)
	Distinct() *DatastoreQuery
	End(c aeds.Cursor) *DatastoreQuery
	EventualConsistency() *DatastoreQuery
	Filter(filterStr string, value interface{}) *DatastoreQuery
	First(dst interface{}) (*aeds.Key, bool, error)
	GetAll(dst interface{}) ([]*aeds.Key, error)
	KeysOnly() *DatastoreQuery
	Limit(limit int) *DatastoreQuery
	Offset(offset int) *DatastoreQuery
	Order(fieldName string) *DatastoreQuery
	Project(fieldNames ...string) *DatastoreQuery
	Run() *aeds.Iterator
	Start(c aeds.Cursor) *DatastoreQuery
}

type DatastoreQuery struct {
	Context   appengine.Context
	Query     *aeds.Query
	Datastore *Datastore
}

func NewQuery(db *Datastore, kind string) *DatastoreQuery {
	q := new(DatastoreQuery)
	q.Context = db.Context
	q.Datastore = db
	q.Query = aeds.NewQuery(kind)
	return q
}

func (q *DatastoreQuery) Ancestor(ancestor Key) *DatastoreQuery {
	q.Query = q.Query.Ancestor(ancestor.(*aeds.Key))
	return q
}

func (q *DatastoreQuery) Count() (int, error) {
	return q.Query.Count(q.Context)
}

func (q *DatastoreQuery) Distinct() *DatastoreQuery {
	q.Query = q.Query.Distinct()
	return q
}

func (q *DatastoreQuery) End(c aeds.Cursor) *DatastoreQuery {
	q.Query = q.Query.End(c)
	return q
}

func (q *DatastoreQuery) EventualConsistency() *DatastoreQuery {
	q.Query = q.Query.EventualConsistency()
	return q
}

func (q *DatastoreQuery) Filter(filterStr string, value interface{}) *DatastoreQuery {
	q.Query = q.Query.Filter(filterStr, value)
	return q
}

func (q *DatastoreQuery) GetAll(dst interface{}) ([]*aeds.Key, error) {
	keys, err := q.Query.GetAll(q.Context, dst)
	err = IgnoreFieldMismatch(err)
	return keys, err
}

func (q *DatastoreQuery) KeysOnly() *DatastoreQuery {
	q.Query = q.Query.KeysOnly()
	return q
}

func (q *DatastoreQuery) Limit(limit int) *DatastoreQuery {
	q.Query = q.Query.Limit(limit)
	return q
}

func (q *DatastoreQuery) Offset(offset int) *DatastoreQuery {
	q.Query = q.Query.Offset(offset)
	return q
}

func (q *DatastoreQuery) Order(fieldName string) *DatastoreQuery {
	q.Query = q.Query.Order(fieldName)
	return q
}

func (q *DatastoreQuery) Project(fieldNames ...string) *DatastoreQuery {
	q.Query = q.Query.Project(fieldNames...)
	return q
}

func (q *DatastoreQuery) Run() *aeds.Iterator {
	return q.Query.Run(q.Context)
}

func (q *DatastoreQuery) Start(c aeds.Cursor) *DatastoreQuery {
	q.Query = q.Query.Start(c)
	return q
}

func (q *DatastoreQuery) First(dst interface{}) (*aeds.Key, bool, error) {
	t := q.Limit(1).Run()
	key, err := t.Next(dst)

	// Ignore field mismatch if set
	err = IgnoreFieldMismatch(err)

	// Nothing found
	if err == aeds.Done {
		return key, false, nil
	}

	// Something went wrong
	if err != nil {
		return nil, false, err
	}

	// Success :)
	return key, true, nil
}
