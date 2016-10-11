package mixin

import (
	"fmt"
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

	log.Debug("Getting coupon for code or id '%s'", id, ctx)

	if ok, _ := m.Query().Filter("Code=", code).Get(); ok {
		log.Debug("Found coupon using code '%s'", code, ctx)
		return m.Key(), true, nil
	} else {
		// Get ids from coupon id
		ids := hashid.Decode(id)

		if len(ids) == 0 {
			log.Warn("Unable to decode coupon code '%s'", id, ctx)
			return nil, false, fmt.Errorf("Unable to decode coupon id: %v", id)
		}

		// Recreate coupon key
		key := db.KeyFromInt(m.Kind(), ids[0])

		// Fetch coupon using key
		err := m.Get(key)
		if err != nil {
			log.Warn("Unable to find coupon by key: %v", err, ctx)
			return nil, false, datastore.ErrNoSuchEntity
		}

		// Set RawCode on fetched entity in case this was not parsed from JSON
		v := reflect.ValueOf(m.Entity).Elem().FieldByName("RawCode")
		ptr := v.Addr().Interface().(*string)
		*ptr = id

		log.JSON("Found coupon", m, ctx)

		return m.Key(), true, nil
	}
}

// Get order from id
func orderFromId(m *Model, id string) (datastore.Key, bool, error) {
	db := m.Db
	key := db.KeyFromInt("order", id)

	ok, _ := m.Query().Filter("__key__ =", key).Get()
	if !ok {
		return nil, false, datastore.ErrNoSuchEntity
	}
	return m.Key(), true, nil
}
