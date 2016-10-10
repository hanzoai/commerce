package iface

import (
	aeds "appengine/datastore"
)

type Query interface {
	Ancestor(ancestor Key) Query
	Count() (int, error)
	Distinct() Query
	EventualConsistency() Query
	Filter(filterStr string, value interface{}) Query
	KeysOnly() Query
	Limit(limit int) Query
	Offset(offset int) Query
	Order(fieldName string) Query
	Project(fieldNames ...string) Query
	Run() *aeds.Iterator
	Start(c aeds.Cursor) Query
	End(c aeds.Cursor) Query
	ByKey(key Key, dst interface{}) (*aeds.Key, bool, error)
	ById(id string, dst interface{}) (*aeds.Key, bool, error)
	IdExists(id string) (*aeds.Key, bool, error)
	KeyExists(key Key) (bool, error)
	First(dst interface{}) (*aeds.Key, bool, error)
	FirstKey() (*aeds.Key, bool, error)
	GetAll(dst interface{}) ([]*aeds.Key, error)
	GetKeys() ([]*aeds.Key, error)
}
