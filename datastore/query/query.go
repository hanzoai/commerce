package query

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"google.golang.org/appengine"
	aeds "appengine/datastore"

	"github.com/qedus/nds"

	"hanzo.io/datastore/iface"
	"hanzo.io/datastore/key"
	"hanzo.io/util/log"

	. "hanzo.io/datastore/utils"
)

type Id struct {
	Id_ string
}

type Query struct {
	ctx   context.Context
	aedsq *aeds.Query
	kind  string
}

func New(ctx context.Context, kind string) iface.Query {
	q := new(Query)
	q.ctx = ctx
	q.kind = kind
	q.aedsq = aeds.NewQuery(kind)
	return q
}

// Wrap all App Engine query methods
func (q *Query) Ancestor(ancestor iface.Key) iface.Query {
	q.aedsq = q.aedsq.Ancestor(ancestor.(*aeds.Key))
	return q
}

func (q *Query) Count() (int, error) {
	return q.aedsq.Count(q.ctx)
}

func (q *Query) Distinct() iface.Query {
	q.aedsq = q.aedsq.Distinct()
	return q
}

func (q *Query) EventualConsistency() iface.Query {
	q.aedsq = q.aedsq.EventualConsistency()
	return q
}

func (q *Query) Filter(filterStr string, value interface{}) iface.Query {
	q.aedsq = q.aedsq.Filter(filterStr, value)
	return q
}

func (q *Query) KeysOnly() iface.Query {
	q.aedsq = q.aedsq.KeysOnly()
	return q
}

func (q *Query) Limit(limit int) iface.Query {
	q.aedsq = q.aedsq.Limit(limit)
	return q
}

func (q *Query) Offset(offset int) iface.Query {
	q.aedsq = q.aedsq.Offset(offset)
	return q
}

func (q *Query) Order(fieldName string) iface.Query {
	q.aedsq = q.aedsq.Order(fieldName)
	return q
}

func (q *Query) Project(fieldNames ...string) iface.Query {
	q.aedsq = q.aedsq.Project(fieldNames...)
	return q
}

func (q *Query) Run() *aeds.Iterator {
	return q.aedsq.Run(q.ctx)
}

func (q *Query) Start(c aeds.Cursor) iface.Query {
	q.aedsq = q.aedsq.Start(c)
	return q
}

func (q *Query) End(c aeds.Cursor) iface.Query {
	q.aedsq = q.aedsq.End(c)
	return q
}

// Returns true if entity with key is persisted to datastore
func (q *Query) KeyExists(key iface.Key) (bool, error) {
	_, ok, err := q.KeysOnly().ByKey(key, nil)
	return ok, err
}

// Returns true if entity with key that encodes to id is persisted to datastore
func (q *Query) IdExists(id string) (*aeds.Key, bool, error) {
	return q.KeysOnly().ById(id, nil)
}

// Fetches first entity
func (q *Query) First(dst interface{}) (*aeds.Key, bool, error) {
	// Run query with iterator
	key, err := q.Limit(1).Run().Next(dst)

	// Nothing found
	if key == nil {
		return nil, false, nil
	}

	// Error trying run query
	if IgnoreFieldMismatch(err) != nil {
		return nil, false, err
	}

	// Found it
	return key, true, nil
}

// Fetches first key
func (q *Query) FirstKey() (*aeds.Key, bool, error) {
	return q.KeysOnly().First(nil)
}

// Fetches keys only
func (q *Query) GetKeys() ([]*aeds.Key, error) {
	return q.KeysOnly().GetAll(nil)
}

// Fetches entities. Dst must have type *[]S or *[]*S or *[]P, for some
// struct type S or some non- interface, non-pointer type P such that P
// or *P implements PropertyLoadSaver.
func (q *Query) GetAll(dst interface{}) ([]*aeds.Key, error) {
	v := reflect.ValueOf(dst)
	if dst != nil && !isPtrSlice(v) {
		return nil, fmt.Errorf("Expected dst to be a pointer to a slice or nil, got: %v", v.Kind())
	}

	keys, err := q.aedsq.GetAll(q.ctx, dst)
	err = IgnoreFieldMismatch(err)
	return keys, err
}

func (q *Query) ByKey(key iface.Key, dst interface{}) (*aeds.Key, bool, error) {
	aekey, _ := key.(*aeds.Key)

	if dst == nil {
		dst = &Id{}
	}

	err := nds.Get(q.ctx, aekey, dst)

	// Completely ignore this as we may be querying just for Id{}
	err = ReallyIgnoreFieldMismatch(err)

	// Not found
	if err == aeds.ErrNoSuchEntity {
		return nil, false, nil
	}

	// Query failed for some reason
	if err != nil {
		log.Warn("Failed to query by key: %v", err)
		return nil, false, err
	}

	// Success
	return aekey, true, nil
}

// Query for entity by id
func (q *Query) ById(id string, dst interface{}) (*aeds.Key, bool, error) {
	// Assume encoded key
	k, err := key.Decode(q.ctx, id)

	// Try to fetch by key (can fail in rare edge cases
	if err == nil {
		if k, ok, _ := q.ByKey(k, dst); ok {
			return k, true, nil
		}
	}

	// Try to find by filter
	filter := ""

	// Use unique filter based on model type
	switch q.kind {
	case "store", "product", "collection":
		filter = "Slug="
	case "variant":
		filter = "SKU="
	case "organization", "mailinglist":
		filter = "Name="
	case "aggregate":
		filter = "Instance="
	case "site":
		filter = "Name="
	case "namespace":
		filter = "Name="
	case "user":
		if strings.Contains(id, "@") {
			filter = "Email="
		} else {
			filter = "Username="
		}
	case "referrer":
		filter = "Code="
	case "coupon":
		return q.couponFromId(id, dst)
	case "order":
		return q.orderFromId(id, dst)
	default:
		return nil, false, errors.New(fmt.Sprintf("Not a valid kind for query: '%s'\nDecode error: '%s'", q.kind, err))
	}

	// Query by filter last
	return q.Filter(filter, id).First(dst)
}
