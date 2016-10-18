package query

import (
	"reflect"
	"strings"

	aeds "appengine/datastore"

	"crowdstart.com/util/hashid"

	"crowdstart.com/datastore/key"
)

// Get coupon from id
func (q *Query) couponFromId(id string, dst interface{}) (*aeds.Key, bool, error) {
	code := strings.ToUpper(id)

	if k, ok, _ := q.Filter("Code=", code).First(dst); ok {
		return k, true, nil
	} else {
		// Get ids from coupon id
		ids := hashid.Decode(id)

		if len(ids) == 0 {
			return nil, false, ErrInvalidKey
		}

		// Recreate coupon key
		k := key.NewFromInt(q.ctx, "coupon", ids[0], nil)

		// Fetch coupon using key
		_, ok, err := q.ByKey(k, dst)

		if err != nil {
			return nil, false, err
		}

		if !ok {
			return nil, false, nil
		}

		// Set RawCode on fetched entity in case this was not parsed from JSON
		v := reflect.ValueOf(dst).Elem().FieldByName("RawCode")
		ptr := v.Addr().Interface().(*string)
		*ptr = id

		return k, true, nil
	}
}

// Get order from id
func (q *Query) orderFromId(id string, dst interface{}) (*aeds.Key, bool, error) {
	k := key.NewFromInt(q.ctx, "order", id, nil)
	k, ok, err := q.ByKey(k, dst)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	return k, ok, err
}
