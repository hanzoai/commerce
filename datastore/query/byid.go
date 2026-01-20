package query

import (
	"strconv"
	"strings"

	aeds "google.golang.org/appengine/datastore"

	"github.com/hanzoai/commerce/util/hashid"
	"github.com/hanzoai/commerce/util/reflect"

	"github.com/hanzoai/commerce/datastore/key"
)

var newKeyFromInt = key.NewFromInt

// Get coupon from id
func (q *Query) couponFromId(id string, dst interface{}) (*aeds.Key, bool, error) {
	code := strings.ToUpper(id)

	key, ok, err := q.Filter("Code=", code).First(dst)
	if ok {
		return key, true, nil
	}
	if err != nil {
		return nil, false, err
	}

	// Get ids from coupon id
	ids, err := hashid.Decode(id)
	if err != nil {
		return nil, false, ErrInvalidKey
	}

	if len(ids) != 3 || ids[0] != 3333 {
		return nil, false, ErrInvalidKey
	}

	// Recreate coupon key
	key, err = newKeyFromInt(q.ctx, "coupon", ids[1], nil)
	if err != nil {
		return nil, false, err
	}

	// Fetch coupon using key
	_, ok, err = q.ByKey(key, dst)
	if err != nil {
		return nil, false, err
	}

	if !ok {
		return nil, false, nil
	}

	// Set RawCode on fetched entity in case this was not parsed from JSON
	reflect.SetField(reflect.ValueOf(dst), "RawCode", id)

	return key, true, nil
}

// Get order from id
func (q *Query) orderFromId(id string, dst interface{}) (*aeds.Key, bool, error) {
	// Coerce into number type
	n, err := strconv.Atoi(id)
	if err != nil {
		return nil, false, err
	}

	key, ok, err := q.Filter("Number=", n).First(dst)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}

	return key, ok, err
}
