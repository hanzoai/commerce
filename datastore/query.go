package datastore

import (
	"appengine"
	aeds "appengine/datastore"
)

type Query interface {
	Ancestor(ancestor Key) Query
	Count() (int, error)
	Distinct() Query
	End(c aeds.Cursor) Query
	EventualConsistency() Query
	Filter(filterStr string, value interface{}) Query
	GetAll(dst interface{}) ([]*aeds.Key, error)
	KeysOnly() Query
	Limit(limit int) Query
	Offset(offset int) Query
	Order(fieldName string) Query
	Project(fieldNames ...string) Query
	Run() *aeds.Iterator
	Start(c aeds.Cursor) Query
	First(dst interface{}) (*aeds.Key, bool, error)
}

type query struct {
	Context   appengine.Context
	Query     *aeds.Query
	Datastore *Datastore
}

func NewQuery(kind string, db *Datastore) *query {
	q := new(query)
	q.Context = db.Context
	q.Datastore = db
	q.Query = aeds.NewQuery(kind)
	return q
}

func (q *query) Ancestor(ancestor Key) Query {
	q.Query = q.Query.Ancestor(ancestor.(*aeds.Key))
	return q
}

func (q *query) Count() (int, error) {
	return q.Query.Count(q.Context)
}

func (q *query) Distinct() Query {
	q.Query = q.Query.Distinct()
	return q
}

func (q *query) End(c aeds.Cursor) Query {
	q.Query = q.Query.End(c)
	return q
}

func (q *query) EventualConsistency() Query {
	q.Query = q.Query.EventualConsistency()
	return q
}

func (q *query) Filter(filterStr string, value interface{}) Query {
	q.Query = q.Query.Filter(filterStr, value)
	return q
}

func (q *query) GetAll(dst interface{}) ([]*aeds.Key, error) {
	keys, err := q.Query.GetAll(q.Context, dst)
	err = q.Datastore.SkipFieldMismatch(err)
	return keys, err
}

func (q *query) KeysOnly() Query {
	q.Query = q.Query.KeysOnly()
	return q
}

func (q *query) Limit(limit int) Query {
	q.Query = q.Query.Limit(limit)
	return q
}

func (q *query) Offset(offset int) Query {
	q.Query = q.Query.Offset(offset)
	return q
}

func (q *query) Order(fieldName string) Query {
	q.Query = q.Query.Order(fieldName)
	return q
}

func (q *query) Project(fieldNames ...string) Query {
	q.Query = q.Query.Project(fieldNames...)
	return q
}

func (q *query) Run() *aeds.Iterator {
	return q.Query.Run(q.Context)
}

func (q *query) Start(c aeds.Cursor) Query {
	q.Query = q.Query.Start(c)
	return q
}

func (q *query) First(dst interface{}) (*aeds.Key, bool, error) {
	t := q.Limit(1).Run()
	key, err := t.Next(dst)

	// Ignore field mismatch if set
	if err != nil {
		// Nothing found
		if err == aeds.Done {
			return key, false, nil
		}

		// Ignore field mismatch, or return error
		if err = q.Datastore.SkipFieldMismatch(err); err != nil {
			return key, false, err
		}
	}

	// Success :)
	return key, true, err
}
