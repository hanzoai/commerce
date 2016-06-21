package mixin

import (
	"reflect"
	"strings"

	"crowdstart.com/datastore"
	"crowdstart.com/util/hashid"
	"crowdstart.com/util/log"
)

// Get coupon from id
func couponFromId(m *Model, id string) (datastore.Key, bool, error) {
	db := m.Db
	ctx := m.Context()
	code := strings.ToUpper(id)

	log.Warn("GETBYIDCODE: %v", code, ctx)

	if ok, _ := m.Query().Filter("Code=", code).First(); ok {
		log.Warn("FOUND KEY", ctx)
		return m.Key(), true, nil
	} else {
		// Get ids from unique coupon code
		ids := hashid.Decode(id)

		// Recreate coupon key
		key := db.KeyFromInt(m.Kind(), ids[0])

		// Fetch coupon using key
		err := m.Get(key)
		if err != nil {
			log.Warn("Unable to filter by key for coupon: %v", err, ctx)
			return nil, false, datastore.KeyNotFound
		}

		// Set RawCode on fetched entity in case this was not parsed from JSON
		v := reflect.ValueOf(m.Entity).Elem().FieldByName("RawCode")
		ptr := v.Addr().Interface().(*string)
		*ptr = id

		return m.Key(), true, nil
	}
}

// Get order from id
func orderFromId(m *Model, id string) (datastore.Key, bool, error) {
	db := m.Db
	key := db.KeyFromInt("order", id)

	ok, _ := m.Query().Filter("__key__ =", key).First()
	if !ok {
		return nil, false, datastore.KeyNotFound
	}
	return m.Key(), true, nil
}
