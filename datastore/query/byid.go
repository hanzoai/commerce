package query

import (
	"strings"

	aeds "appengine/datastore"

	"crowdstart.com/util/hashid"
	"crowdstart.com/util/reflect"

	"crowdstart.com/datastore/key"
)

var newKeyFromInt = key.NewFromInt

// Get coupon from id
func (q *Query) couponFromId(id string, dst interface{}) (*aeds.Key, bool, error) {
	code := strings.ToUpper(id)

	key, ok, _ := q.Filter("Code=", code).First(dst)
	if ok {
		return key, true, nil
	}

	// Get ids from coupon id
	ids, err := hashid.Decode(id)
	if err != nil {
		return nil, false, ErrInvalidKey
	}

	if len(ids) == 0 {
		return nil, false, ErrInvalidKey
	}

	// Recreate coupon key
	key, err = newKeyFromInt(q.ctx, "coupon", ids[0], nil)
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
	reflect.SetField(dst, "RawCode", id)

	return key, true, nil
}

// Get order from id
func (q *Query) orderFromId(id string, dst interface{}) (*aeds.Key, bool, error) {
	key, err := newKeyFromInt(q.ctx, "order", id, nil)
	if err != nil {
		return nil, false, err
	}

	key, ok, err := q.ByKey(key, dst)
	if err != nil {
		return nil, false, err
	}

	if !ok {
		return nil, false, nil
	}

	return key, ok, err
}
